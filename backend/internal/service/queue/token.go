package queue

import (
	"crypto/rand"
	"encoding/hex"
)

const tokenPrefix = "rgt_"

// newToken carries no data inside the token — ownership is established by looking
// it up, so nothing in it can be forged (docs/design_context.md, «Модель данных»).
func newToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("queue: cannot read random bytes: " + err.Error())
	}

	return tokenPrefix + hex.EncodeToString(buf)
}
