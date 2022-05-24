package deployment

import (
	"fmt"
	"sync"
)

type Source struct {
	id  string
	dir string
}

func NewSource(dir string) Source {
	return Source{
		id:  getNextSourceId(),
		dir: dir,
	}
}

var nextSourceId int
var nextSourceIdMutex sync.Mutex

func getNextSourceId() string {
	nextSourceIdMutex.Lock()
	defer nextSourceIdMutex.Unlock()
	id := fmt.Sprintf("%d", nextSourceId)
	nextSourceId += 1
	return id
}
