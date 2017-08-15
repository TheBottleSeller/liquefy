package models

import (
	"bytes"
	"encoding/gob"
)

type StatusMessage struct {
	ContainerJob ContainerJob
	StatusMessage string
}


func SerializeStatusMessage(im *StatusMessage) ([]byte, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(im)
	if err != nil {
		return nil,NewError("Failed to serialize Interchange Message",err)
	}
	return buf.Bytes(), err
}

func DeserializeStatusMessage(content []byte) (*StatusMessage, error) {
	im := StatusMessage{}
	buf := bytes.Buffer{}
	buf.Write(content)
	dec := gob.NewDecoder(&buf)
	err := dec.Decode(&im)
	if err != nil {
		return nil,NewError("Failed to de-serialize Interchange Message",err)
	}
	return &im, err
}
