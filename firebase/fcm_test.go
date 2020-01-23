package firebase

import (
	"os"
	"testing"
)

const token = "foIh7-NdlksspjDwT8O5kT:APA91bEQUCFeAadkIE-T3fHqAIIYwZm8lks0wQRIp5oh0qtMtjHcPjQhVZ3IDntZlv7PYAcHvDeu_7ncI8GcAlKama7YjzSLO9MgtAjxZMFivVfzQb-BD-6u0-MrJNR6XoOB9YX059ZB"

func TestSendNotification(t *testing.T) {
	dir, _ := os.Getwd()
	_, err := SendNotification([]byte(token), dir+"/../creds/serviceAccountKey.json")
	if err != nil {
		t.Error(err.Error())
	}
}
