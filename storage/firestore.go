package storage

import (
	"context"

	"cloud.google.com/go/firestore"
)

type FirestoreDriver struct {
	ctx    context.Context
	client *firestore.Client
}

func (fs *FirestoreDriver) Init(projectID string) error {
	fs.ctx = context.TODO()
	client, err := createFirestoreClient(fs.ctx, projectID)
	if err != nil {
		return err
	}
	fs.client = client
	return nil
}

func (fs *FirestoreDriver) Close() error {
	return fs.client.Close()
}

func (fs *FirestoreDriver) GetGuildData(string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func createFirestoreClient(ctx context.Context, projectID string) (*firestore.Client, error) {
	// Sets your Google Cloud Platform project ID.
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// Close client when done with
	// defer client.Close()

	return client, nil
}
