package server

import (
	"fmt"
	"net/http"
	"testing"
)

func TestRejectedRoomChangesSaveNothing(t *testing.T) {
	server := newTestServer(t)
	client := server.claimed()

	refused := client.do(http.MethodPost, "/api/v1/groups/rooms", map[string]any{"name": "Study", "description": "Desk", "photoId": 999999})
	if refused.code() != "invalid_photo" {
		t.Fatalf("bad photo: %d %s", refused.Status, refused.Body)
	}
	var rooms int
	server.application.Store.Database.QueryRow(`SELECT count(*) FROM rooms WHERE name = 'Study'`).Scan(&rooms)
	if rooms != 0 {
		t.Fatal("a refused create left the room behind")
	}
	var photo struct {
		ID int64 `json:"id"`
	}
	client.upload("photo", "study.png", "image/png", pngBytes(10, 10)).json(t, &photo)
	created := client.do(http.MethodPost, "/api/v1/groups/rooms", map[string]any{"name": "Study", "description": "Desk", "photoId": photo.ID})
	if created.Status != http.StatusCreated {
		t.Fatalf("a corrected retry failed: %d %s", created.Status, created.Body)
	}
	var room struct {
		ID int64 `json:"id"`
	}
	created.json(t, &room)

	patched := client.do(http.MethodPatch, fmt.Sprintf("/api/v1/groups/rooms/%d", room.ID), map[string]any{"name": "Office", "description": "Changed", "photoId": 999999})
	if patched.code() != "invalid_photo" {
		t.Fatalf("bad photo on edit: %d %s", patched.Status, patched.Body)
	}
	var name, description string
	server.application.Store.Database.QueryRow(`SELECT name, description FROM rooms WHERE id = ?`, room.ID).Scan(&name, &description)
	if name != "Study" || description != "Desk" {
		t.Fatalf("a refused edit partly saved: %q %q", name, description)
	}
}
