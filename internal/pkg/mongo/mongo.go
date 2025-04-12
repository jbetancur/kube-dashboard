package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/jbetancur/dashboard/internal/pkg/storage"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Store is a simplified MongoDB client for storing Kubernetes resources
type Store struct {
	client     *mongo.Client
	collection *mongo.Collection
	logger     *slog.Logger
}

// ResourceMetadata contains common Kubernetes resource metadata
type ResourceMetadata struct {
	Kind            string
	APIVersion      string
	Name            string
	Namespace       string
	ResourceVersion string
	UID             string
}

// NewStore creates a new MongoDB store
func NewStore(ctx context.Context, uri, database string, logger *slog.Logger) (storage.Repository, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Create the collection
	collection := client.Database(database).Collection("resources")

	// First, drop any existing problematic indexes to avoid conflicts
	_, err = collection.Indexes().DropAll(ctx)
	if err != nil {
		logger.Warn("Failed to drop existing indexes (this is usually fine for first run)", "error", err)
		// Continue anyway - this may fail on first run when no indexes exist
	}

	// Create indexes for efficient querying - without uniqueness on UID
	_, err = collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "cluster_id", Value: 1},
				{Key: "kind", Value: 1},
				{Key: "namespace", Value: 1},
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "uid", Value: 1}},
			Options: options.Index(), // No uniqueness constraint on UID
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &Store{
		client:     client,
		collection: collection,
		logger:     logger,
	}, nil
}

// Save stores any Kubernetes resource in MongoDB
func (s *Store) Save(ctx context.Context, clusterID string, obj runtime.Object) error {
	// Extract metadata using unstructured conversion
	meta, err := extractMetadata(obj)
	if err != nil {
		return fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Handle missing values
	if meta.Kind == "" {
		s.logger.Warn("Missing kind in resource, using fallback")
	}

	if meta.Name == "" {
		return fmt.Errorf("resource must have a name")
	}

	// Generate a proper document ID
	var id string = fmt.Sprintf("%s:%s:%s:%s", clusterID, meta.Namespace, meta.Kind, meta.Name)
	if meta.Kind == "Namespace" {
		id = fmt.Sprintf("%s:%s:%s", clusterID, meta.Kind, meta.Name)
	}

	// Handle missing UID
	uid := meta.UID
	if uid == "" {
		uid = fmt.Sprintf("%s-%s-%s", clusterID, meta.Namespace, meta.Name)
		s.logger.Debug("Generated synthetic UID for resource",
			"kind", meta.Kind,
			"name", meta.Name,
			"uid", uid)
	}

	// Ensure TypeMeta fields are properly set in the object
	// This helps with correctly storing kind and apiVersion
	if metaAccessor, ok := obj.(interface {
		GetObjectKind() schema.ObjectKind
	}); ok {
		gvk := schema.GroupVersionKind{
			Kind: meta.Kind,
		}

		// Handle API version correctly
		if meta.APIVersion != "" {
			if strings.Contains(meta.APIVersion, "/") {
				parts := strings.Split(meta.APIVersion, "/")
				gvk.Group = parts[0]
				gvk.Version = parts[1]
			} else {
				gvk.Version = meta.APIVersion
			}
		} else {
			gvk.Version = "v1" // Default for core resources
		}

		metaAccessor.GetObjectKind().SetGroupVersionKind(gvk)
	}

	// Create the document with all necessary fields
	doc := bson.M{
		"_id":              id,
		"cluster_id":       clusterID,
		"kind":             meta.Kind,
		"api_version":      meta.APIVersion,
		"name":             meta.Name,
		"uid":              uid,
		"resource_version": meta.ResourceVersion,
		"resource":         obj,
		"updated_at":       time.Now(),
	}

	// Only include namespace field for namespaced resources
	if meta.Kind != "Namespace" {
		doc["namespace"] = meta.Namespace
	}

	// Upsert the document (create if not exists, update if exists)
	opts := options.Update().SetUpsert(true)
	_, err = s.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{
			"$set":         doc,
			"$setOnInsert": bson.M{"created_at": time.Now()},
		},
		opts,
	)

	if err != nil {
		return fmt.Errorf("failed to save resource: %w", err)
	}

	return nil
}

// Get retrieves a Kubernetes resource by its identifying information
func (s *Store) Get(ctx context.Context, clusterID, namespace, kind, name string, result interface{}) error {
	// Generate the correct ID based on resource type
	var id string
	if kind == "Namespace" {
		id = fmt.Sprintf("%s:%s:%s", clusterID, kind, name)
	} else {
		id = fmt.Sprintf("%s:%s:%s:%s", clusterID, namespace, kind, name)
	}

	// Find the document by ID
	var doc bson.M
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("resource not found: %s", id)
		}
		return fmt.Errorf("database error: %w", err)
	}

	// Extract the resource field from the document
	resourceData, ok := doc["resource"]
	if !ok {
		return fmt.Errorf("resource data not found in document")
	}

	// Convert the resource data to the requested type
	resourceBytes, err := bson.Marshal(resourceData)
	if err != nil {
		return fmt.Errorf("failed to marshal resource data: %w", err)
	}

	// Unmarshal into the provided result
	return bson.Unmarshal(resourceBytes, result)
}

// List returns all resources of a specific kind
func (s *Store) List(ctx context.Context, clusterID, namespace, kind string, results interface{}) error {
	// Construct filter based on inputs
	filter := bson.M{
		"cluster_id": clusterID,
		"kind":       kind,
	}

	if namespace != "" && namespace != "all" {
		filter["namespace"] = namespace
	}

	// Find matching documents
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}
	defer cursor.Close(ctx)

	// Get the slice value that results points to
	resultsVal := reflect.ValueOf(results)
	if resultsVal.Kind() != reflect.Ptr || resultsVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("results must be a pointer to slice")
	}

	// Intermediate results to store documents
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return fmt.Errorf("failed to read cursor: %w", err)
	}

	// Create a slice of the correct type
	sliceType := resultsVal.Elem().Type()
	elemType := sliceType.Elem()
	slice := reflect.MakeSlice(sliceType, 0, len(docs))

	// Extract and decode each resource
	for _, doc := range docs {
		resourceData, ok := doc["resource"]
		if !ok {
			s.logger.Warn("Found document without resource field", "id", doc["_id"])
			continue
		}

		// Create new element of the slice's element type
		elemPtr := reflect.New(elemType)

		// Marshal and unmarshal to convert
		resourceBytes, err := bson.Marshal(resourceData)
		if err != nil {
			s.logger.Error("Failed to marshal resource", "error", err)
			continue
		}

		if err := bson.Unmarshal(resourceBytes, elemPtr.Interface()); err != nil {
			s.logger.Error("Failed to unmarshal resource", "error", err)
			continue
		}

		// Append the element (not the pointer) to our slice
		slice = reflect.Append(slice, elemPtr.Elem())
	}

	// Assign the new slice to the results pointer
	resultsVal.Elem().Set(slice)
	return nil
}

// Delete removes a resource
func (s *Store) Delete(ctx context.Context, clusterID, namespace, kind, name string) error {
	// Generate the correct ID
	var id string
	if kind == "Namespace" {
		id = fmt.Sprintf("%s:%s:%s", clusterID, kind, name)
	} else {
		id = fmt.Sprintf("%s:%s:%s:%s", clusterID, namespace, kind, name)
	}

	_, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	return nil
}

// DeleteByFilter removes resources matching a filter
func (s *Store) DeleteByFilter(ctx context.Context, filter map[string]interface{}) error {
	_, err := s.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete resources: %w", err)
	}
	return nil
}

// Close closes the MongoDB connection
func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// extractMetadata extracts common metadata from a Kubernetes resource
func extractMetadata(obj runtime.Object) (ResourceMetadata, error) {
	metadata := ResourceMetadata{}

	// Convert to unstructured - this is the Kubernetes-recommended way
	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return metadata, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	// Create an Unstructured wrapper for convenient accessor methods
	unstructObj := &unstructured.Unstructured{Object: unstruct}

	// Extract all metadata in one go
	metadata.Kind = unstructObj.GetKind()
	metadata.APIVersion = unstructObj.GetAPIVersion()
	metadata.Name = unstructObj.GetName()
	metadata.Namespace = unstructObj.GetNamespace()
	metadata.ResourceVersion = unstructObj.GetResourceVersion()
	metadata.UID = string(unstructObj.GetUID())

	// If Kind is empty, determine it from the object's type
	if metadata.Kind == "" {
		metadata.Kind = getKindFromType(obj)
	}

	// If APIVersion is empty, use reasonable default
	if metadata.APIVersion == "" {
		metadata.APIVersion = "v1" // Default for core resources
	}

	return metadata, nil
}

// getKindFromType determines the Kubernetes Kind from a runtime.Object's Go type
func getKindFromType(obj runtime.Object) string {
	// Get the type name using reflection
	t := reflect.TypeOf(obj)

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Extract just the type name (not the full package path)
	typeName := t.Name()

	// Special cases for known types
	switch typeName {
	case "Pod":
		return "Pod"
	case "Namespace":
		return "Namespace"
	case "Service":
		return "Service"
	case "Deployment":
		return "Deployment"
	case "ConfigMap":
		return "ConfigMap"
	case "Secret":
		return "Secret"
	case "Node":
		return "Node"
	case "PersistentVolume":
		return "PersistentVolume"
	case "PersistentVolumeClaim":
		return "PersistentVolumeClaim"
	case "ReplicaSet":
		return "ReplicaSet"
	case "StatefulSet":
		return "StatefulSet"
	case "DaemonSet":
		return "DaemonSet"
	case "Job":
		return "Job"
	case "CronJob":
		return "CronJob"
	case "Ingress":
		return "Ingress"
	default:
		// For unknown types, use the type name as the Kind
		return typeName
	}
}
