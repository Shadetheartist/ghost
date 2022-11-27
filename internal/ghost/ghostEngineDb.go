package ghost

import (
	"bytes"
	"errors"
	"log"
	"os"

	"encoding/gob"
)

type SaveData struct {
	Requests []Request
}

// saves pending requests to ghostdb file
func (e *Engine) Save() error {

	ghostSaveData := SaveData{}
	ghostSaveData.Requests = make([]Request, 0, len(e.requestMap))

	for _, r := range e.requestMap {
		ghostSaveData.Requests = append(ghostSaveData.Requests, *r)
	}

	// if there's no requests pending, we dont need to save anything
	if len(ghostSaveData.Requests) < 1 {
		return nil
	}

	log.Print("Saving pending requests to file... ")

	file, err := os.Create(e.dbFileLocation)
	if err != nil {
		file.Close()
		return err
	}

	encoder := gob.NewEncoder(file)
	encoder.Encode(&ghostSaveData)

	log.Println("Done!")

	return nil
}

// loads requests from ghostdb file and then deletes the file (if it exists)
func (e *Engine) Load() error {

	// if the file doesn't exist, we don't need to load it
	if _, err := os.Stat(e.dbFileLocation); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	fileBytes, err := os.ReadFile(e.dbFileLocation)
	if err != nil {
		return err
	}

	log.Print("Loading pending requests from file... ")

	ghostSaveData := SaveData{}

	encoder := gob.NewDecoder(bytes.NewReader(fileBytes))
	encoder.Decode(&ghostSaveData)

	for idx := range ghostSaveData.Requests {
		err := e.RegisterRequest(&ghostSaveData.Requests[idx])
		if err != nil {
			log.Printf("Err registering loaded request: %s\n", err.Error())
		}
	}

	os.Remove(e.dbFileLocation)

	log.Println("Done!")

	return nil
}
