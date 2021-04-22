package db

import (
	"context" // State handling across API boundaries; part of native GoLang API
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"cloud.google.com/go/firestore"   // Firestore-specific support
	firebase "firebase.google.com/go" // Generic firebase support
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Firebase context and client used by Firestore functions throughout the program.
var ctx context.Context
var client *firestore.Client

// Collection name in Firestore
var collection = "Subscriptions"

// Message counter to produce some variation in content
var ct = 0

// Tasks:
// - Allow selection of individual messages, as opposed to all
// - Introduce update functionality via PUT and/or PATCH
// - Adapt addMessage and displayMessage function to support custom JSON schema

// Lists all the messages in the messages collection to the user.
func displayMessage(w http.ResponseWriter, r *http.Request) {

	iter := client.Collection(collection).Documents(ctx) // Loop through all entries in collection "Subscriptions"

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("Failed to iterate: %v", err)
		}
		m := doc.Data() // A message map with string keys. Each key is one field, like "text" or "timestamp"
		_, err = fmt.Fprintln(w, m)
		if err != nil {
			http.Error(w, "Error while writing response body.", http.StatusInternalServerError)
		}
	}
}

// Reads a string from the body in plain-text and sends it to firestore to be registered as a message.
func addMessage(w http.ResponseWriter, r *http.Request) {
	text, err := ioutil.ReadAll(r.Body) // very generic way of reading body; should be customized to specific use case
	if err != nil {
		http.Error(w, "Reading of payload failed", http.StatusInternalServerError)
	}
	fmt.Println("Received message ", string(text))
	if len(string(text)) == 0 {
		http.Error(w, "Your message appears to be empty. Ensure to terminate URI with /.", http.StatusBadRequest)
	} else {
		id, _, err := client.Collection("messages").Add(ctx,
			map[string]interface{}{
				"text": string(text),
				"ct":   ct,
				"time": firestore.ServerTimestamp,
			})
		ct++
		if err != nil {
			http.Error(w, "Error when adding message "+string(text), http.StatusBadRequest)
		} else {
			fmt.Println("Entry added to collection.")
			http.Error(w, id.ID, http.StatusCreated) // Returns document ID
		}
	}
}

// Deletes an individual or all messages from firestore
func deleteMessage(w http.ResponseWriter, r *http.Request) {

	// extract message ID from URL
	elem := strings.Split(r.URL.Path, "/")
	messageId := elem[2]

	// Delete individual message based on reference
	if len(messageId) != 0 {
		_, err := client.Collection(collection).Doc(messageId).Delete(ctx)
		if err != nil {
			http.Error(w, "Deletion of "+messageId+" failed.", http.StatusInternalServerError)
		}
		http.Error(w, "Deletion of "+messageId+" successful.", http.StatusNoContent)
	} else {
		// Delete all messages
		it := client.Collection(collection).Documents(ctx)
		for {
			item, err := it.Next()
			if err == iterator.Done {
				break
			}
			_, err = item.Ref.Delete(ctx)
			if err != nil {
				http.Error(w, "Deletion of item "+item.Ref.ID+" failed.", http.StatusInternalServerError)
			}
			http.Error(w, "Messages deleted.", http.StatusNoContent)
		}
	}
}

// Handler for all message-related operations
func handleMessage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		addMessage(w, r)
	case http.MethodGet:
		displayMessage(w, r)
	case http.MethodDelete:
		deleteMessage(w, r)
	default:
		http.Error(w, "Unsupported request method", http.StatusMethodNotAllowed)
	}
}

func initDB() {
	// Firebase initialisation
	ctx = context.Background()

	// We use a service account, load credentials file that you downloaded from your project's settings menu.
	// Make sure this file is gitignored, it is the access token to the database.
	sa := option.WithCredentialsFile("service-account.json")
	app, err := firebase.NewApp(ctx, nil, sa)
	if err != nil {
		log.Fatalln(err)
	}

	client, err = app.Firestore(ctx)

	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()
}
