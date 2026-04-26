package mpd

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/fhs/gompd/v2/mpd"
)

var albumArtLock sync.Mutex
var (
	mpdTemp     = filepath.Join(os.TempDir(), "mpd_mpris") // Temp folder location
	albumArtURI string
)

func newTempFile() string {
	if err := os.MkdirAll(mpdTemp, 0777); err != nil {
		log.Println("Cannot create temp file for album art, we don't support them then!", err)
		return ""
	}
	f, err := ioutil.TempFile(mpdTemp, "artwork_")
	if err != nil {
		log.Println("Cannot create temp file for album art, we don't support them then!", err)
		return ""
	}
	defer f.Close()
	return f.Name()
}

// Song represents a music file with metadata.
type Song struct {
	File
	ID int // The song's ID (within the playlist)

	albumArt bool // Whether the song has an album art. The album art will be loaded into memory at AlbumArtURI.
}

// SameAs checks if both songs are the same.
func (s *Song) SameAs(other *Song) bool {
	if other == nil || s == nil {
		return s == nil && other == nil
	}
	return s.ID == other.ID && s.Path() == other.Path() && s.Title == other.Title
}

// SongFromAttrs returns a song from the attributes map.
func (c *Client) SongFromAttrs(attr mpd.Attrs) (s Song, err error) {
	if s.ID, err = strconv.Atoi(attr["Id"]); err != nil {
		s.ID = -1
		return s, nil
	}
	if s.File, err = c.FileFromAttrs(attr); err != nil {
		return
	}

	// Attempt to load the album art.
	albumArtLock.Lock()
	defer albumArtLock.Unlock()

	art, err := c.getAlbumArt(s.Path())
	if err != nil {
		log.Println(err)
		return s, nil
	}
	if len(art) == 0 {
		return s, nil
	}

	newAlbumArtURI := newTempFile()
	if newAlbumArtURI == "" {
		return s, nil
	}
	if err := ioutil.WriteFile(newAlbumArtURI, art, 0644); err != nil {
		log.Println(err)
		os.Remove(newAlbumArtURI)
		return s, nil
	}

	if albumArtURI != "" {
		// delete the old album art file
		os.Remove(albumArtURI)
	}
	albumArtURI = newAlbumArtURI
	s.albumArt = true

	return
}

// Get a song's album art, first by trying readpicture, then try albumart.
func (c *Client) getAlbumArt(uri string) ([]byte, error) {
	if art, err := c.readPicture(uri); err == nil {
		return art, nil
	}
	return c.AlbumArt(uri)
}

// readPicture retrieves an album artwork image for a song with the given URI using MPD's readpicture command.
// Pretty much the same as `c.AlbumArt`.
func (c *Client) readPicture(uri string) ([]byte, error) {
	offset := 0
	var data []byte
	for {
		// Read the data in chunks
		chunk, size, err := c.Command("readpicture %s %d", uri, offset).Binary()
		if err != nil {
			return nil, err
		}

		// Accumulate the data
		data = append(data, chunk...)
		offset = len(data)
		if offset >= size {
			break
		}
	}
	return data, nil
}

// AlbumArtURI returns the URI to the album art, if it is available.
func (s Song) AlbumArtURI() (string, bool) {
	if !s.albumArt {
		return "", false
	}
	// Should I do something better here?
	return "file://" + albumArtURI, true
}
