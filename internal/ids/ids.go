package ids

import (
	"crypto/rand"
	"fmt"
)

// Generator implements agentruntime.IDGenerator.
type Generator struct{}

func (Generator) Generate() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
