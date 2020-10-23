package storage

import (
	"context"
	"encoding/json"
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

func (fs *FirestoreDriver) GetGuildSettings(guildID string) (*GuildSettings, error) {
	docs := fs.client.Collection("guildSettings").Where("guildID", "==", guildID).Documents(fs.ctx)
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		data := doc.Data()

		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		settings := GuildSettings{}
		err = json.Unmarshal(bytes, &settings)
		if err != nil {
			return nil, err
		}

		return &settings, nil
	}
}

func (fs *FirestoreDriver) WriteGuildSettings(guildID string, settings *GuildSettings) error {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}
	_, err = fs.client.Collection("guildSettings").Doc(guildID).Set(fs.ctx, data)
	return err
}

func (fs *FirestoreDriver) GetAllUserSettings() *UserSettingsCollection {
	col := MakeUserSettingsCollection()
	docs := fs.client.Collection("userSettings").Documents(fs.ctx)
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			return col
		}
		if err != nil {
			return col
		}
		data := doc.Data()

		bytes, err := json.Marshal(data)
		if err != nil {
			return col
		}
		settings := UserSettings{}
		err = json.Unmarshal(bytes, &settings)
		if err != nil {
			return col
		}

		col.users[settings.UserID] = &settings
	}
}

func (fs *FirestoreDriver) WriteUserSettings(userID string, settings *UserSettings) error {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return err
	}
	_, err = fs.client.Collection("userSettings").Doc(userID).Set(fs.ctx, data)
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
