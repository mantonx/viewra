package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestAudioDBWorkflowStep(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive workflow test in short mode")
	}

	// Step 1: Test artist search (this should work)
	t.Run("Step1_ArtistSearch", func(t *testing.T) {
		apiURL := "https://www.theaudiodb.com/api/v1/json/2"
		artist := "coldplay"
		searchURL := fmt.Sprintf("%s/search.php?s=%s", apiURL, url.QueryEscape(artist))
		
		t.Logf("Testing URL: %s", searchURL)
		
		client := &http.Client{Timeout: 30 * time.Second}
		req, err := http.NewRequest("GET", searchURL, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		
		req.Header.Set("User-Agent", "Viewra AudioDB Enricher Test/1.0.0")
		
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("API request failed: %v", err)
		}
		defer resp.Body.Close()
		
		t.Logf("Response status: %d", resp.StatusCode)
		
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}
		
		t.Logf("Response length: %d bytes", len(bodyBytes))
		
		var artistResponse AudioDBArtistResponse
		if err := json.Unmarshal(bodyBytes, &artistResponse); err != nil {
			t.Fatalf("Failed to decode artist response: %v", err)
		}
		
		if len(artistResponse.Artists) == 0 {
			t.Fatal("No artists found")
		}
		
		artist1 := artistResponse.Artists[0]
		t.Logf("Found artist: %s (ID: %s)", artist1.StrArtist, artist1.IDArtist)
		
		// Step 2: Test albums for this artist
		t.Run("Step2_Albums", func(t *testing.T) {
			albumsURL := fmt.Sprintf("%s/album.php?i=%s", apiURL, artist1.IDArtist)
			t.Logf("Testing albums URL: %s", albumsURL)
			
			time.Sleep(1000 * time.Millisecond) // Rate limiting
			
			req, err := http.NewRequest("GET", albumsURL, nil)
			if err != nil {
				t.Fatalf("Failed to create albums request: %v", err)
			}
			
			req.Header.Set("User-Agent", "Viewra AudioDB Enricher Test/1.0.0")
			
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Albums API request failed: %v", err)
			}
			defer resp.Body.Close()
			
			t.Logf("Albums response status: %d", resp.StatusCode)
			
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read albums response: %v", err)
			}
			
			t.Logf("Albums response length: %d bytes", len(bodyBytes))
			t.Logf("Albums response preview: %s", string(bodyBytes[:minIntCompTest(500, len(bodyBytes))]))
			
			var albumResponse AudioDBAlbumResponse
			if err := json.Unmarshal(bodyBytes, &albumResponse); err != nil {
				t.Fatalf("Failed to decode albums response: %v", err)
			}
			
			t.Logf("Found %d albums", len(albumResponse.Album))
			
			if len(albumResponse.Album) > 0 {
				album1 := albumResponse.Album[0]
				t.Logf("First album: %s (ID: %s)", album1.StrAlbum, album1.IDAlbum)
				
				// Step 3: Test tracks for first album
				t.Run("Step3_Tracks", func(t *testing.T) {
					tracksURL := fmt.Sprintf("%s/track.php?m=%s", apiURL, album1.IDAlbum)
					t.Logf("Testing tracks URL: %s", tracksURL)
					
					time.Sleep(1000 * time.Millisecond) // Rate limiting
					
					req, err := http.NewRequest("GET", tracksURL, nil)
					if err != nil {
						t.Fatalf("Failed to create tracks request: %v", err)
					}
					
					req.Header.Set("User-Agent", "Viewra AudioDB Enricher Test/1.0.0")
					
					resp, err := client.Do(req)
					if err != nil {
						t.Fatalf("Tracks API request failed: %v", err)
					}
					defer resp.Body.Close()
					
					t.Logf("Tracks response status: %d", resp.StatusCode)
					
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Fatalf("Failed to read tracks response: %v", err)
					}
					
					t.Logf("Tracks response length: %d bytes", len(bodyBytes))
					t.Logf("Tracks response preview: %s", string(bodyBytes[:minIntCompTest(200, len(bodyBytes))]))
					
					if len(bodyBytes) == 0 {
						t.Log("Empty tracks response - this might be normal for some albums")
						return
					}
					
					var trackResponse AudioDBTrackResponse
					if err := json.Unmarshal(bodyBytes, &trackResponse); err != nil {
						t.Logf("Failed to decode tracks response: %v", err)
						t.Logf("Raw response: %s", string(bodyBytes))
						return // Don't fail, just log
					}
					
					t.Logf("Found %d tracks", len(trackResponse.Track))
					if len(trackResponse.Track) > 0 {
						track1 := trackResponse.Track[0]
						t.Logf("First track: %s (ID: %s)", track1.StrTrack, track1.IDTrack)
					}
				})
			}
		})
	})
}

func minIntCompTest(a, b int) int {
	if a < b {
		return a
	}
	return b
} 