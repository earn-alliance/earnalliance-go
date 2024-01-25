package earnalliance

import "fmt"

// Identifier represents a user's idenfitier which is a string.
// To remove the identifier from the user, its value should be an empty string.
// Use the IdentifierFrom function to create these with ease.
type Identifier string

// MarshalJSON marshals a Identifier to JSON.
// This will marshal a null if string is empty.
// If it's not, it will marshal the string as normal JSON string.
func (s Identifier) MarshalJSON() ([]byte, error) {
	if s == "" {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, s)), nil
}

// IdentifierFrom creates a new Identifier from s.
func IdentifierFrom(s string) *Identifier {
	return PointerFrom(Identifier(s))
}

// RemoveIdentifier creates a new Identifier with empty string.
// Use this to remove any identifier.
func RemoveIdentifier() *Identifier {
	return PointerFrom(Identifier(""))
}
