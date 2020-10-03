package storage

import (
	"context"
	"google.golang.org/api/iterator"

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

func (fs *FirestoreDriver) GetGuildData(guildID string) (map[string]interface{}, error) {
	docs := fs.client.Collection("guilds").Where("guildID", "==", guildID).Documents(fs.ctx)
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		return doc.Data(), nil
	}

}

func (fs *FirestoreDriver) WriteGuildData(guildID string, data map[string]interface{}) error {
	_, err := fs.client.Collection("guilds").Doc(guildID).Set(fs.ctx, data)
	return err
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
