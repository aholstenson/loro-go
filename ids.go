package loro

// StringContainerId is a string that can be used to get maps, lists, etc from
// a document.
type StringContainerId string

func (c StringContainerId) AsContainerId(containerType ContainerType) ContainerId {
	return ContainerIdRoot{
		Name:          string(c),
		ContainerType: containerType,
	}
}

// AsContainerId converts a string to a ContainerIDLike, which can be used to
// get maps, lists, etc from a document.
func AsContainerId(v string) StringContainerId {
	return StringContainerId(v)
}

var _ ContainerIdLike = StringContainerId("")
