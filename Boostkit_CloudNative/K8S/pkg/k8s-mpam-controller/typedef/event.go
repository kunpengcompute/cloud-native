package typedef

type (
	// EventType is the type of event published by generic publisher
	EventType int8
	// Event is the event published by generic publisher
	Event interface{}
)

const (
	// RAWPODADD means Kubernetes starts a new Pod event
	RAWPODADD EventType = iota
	// RAWPODUPDATE means Kubernetes updates Pod event
	RAWPODUPDATE
	// RAWPODDELETE means Kubernetes deletes Pod event
	RAWPODDELETE
	// INFOADD means PodManager adds pod information event
	INFOADD
	// INFOUPDATE means PodManager updates pod information event
	INFOUPDATE
	// INFODELETE means PodManager deletes pod information event
	INFODELETE
	// RAWPODSYNCALL means Full amount of kubernetes pods
	RAWPODSYNCALL
	// NRIPODADD means nri starts a new Pod event
)

const undefinedType = "undefined"

var eventTypeToString = map[EventType]string{
	RAWPODADD:     "addrawpod",
	RAWPODUPDATE:  "updaterawpod",
	RAWPODDELETE:  "deleterawpod",
	INFOADD:       "addinfo",
	INFOUPDATE:    "updateinfo",
	INFODELETE:    "deleteinfo",
	RAWPODSYNCALL: "syncallrawpods",
}

// String returns the string of the current event type
func (t EventType) String() string {
	if str, ok := eventTypeToString[t]; ok {
		return str
	}
	return undefinedType
}
