package ports

// IDGenerator defines a generator for time-sortable unique identifiers.
type IDGenerator interface {
	NewID() string
}
