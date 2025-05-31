package loro

// LocalUpdateCallbackFn adapts a function to the LocalUpdateCallback interface.
type LocalUpdateCallbackFn func(update []byte)

func (fn LocalUpdateCallbackFn) OnLocalUpdate(update []byte) {
	fn(update)
}

// AsLocalUpdateCallback adapts a function to the LocalUpdateCallback interface.
func AsLocalUpdateCallback(fn LocalUpdateCallbackFn) LocalUpdateCallback {
	return fn
}

type SubscriberFn func(diff DiffEvent)

func (fn SubscriberFn) OnDiff(diff DiffEvent) {
	fn(diff)
}

// AsSubscriber adapts a function to the Subscriber interface.
func AsSubscriber(fn SubscriberFn) Subscriber {
	return fn
}

type FirstCommitFromPeerCallbackFn func(payload FirstCommitFromPeerPayload)

func (fn FirstCommitFromPeerCallbackFn) OnFirstCommitFromPeer(payload FirstCommitFromPeerPayload) {
	fn(payload)
}

// AsFirstCommitFromPeerCallback adapts a function to the FirstCommitFromPeerCallback interface.
func AsFirstCommitFromPeerCallback(fn FirstCommitFromPeerCallbackFn) FirstCommitFromPeerCallback {
	return fn
}

// PreCommitCallbackFn adapts a function to the PreCommitCallback interface.
type PreCommitCallbackFn func(payload PreCommitCallbackPayload)

func (fn PreCommitCallbackFn) OnPreCommit(payload PreCommitCallbackPayload) {
	fn(payload)
}

// AsPreCommitCallback adapts a function to the PreCommitCallback interface.
func AsPreCommitCallback(fn PreCommitCallbackFn) PreCommitCallback {
	return fn
}

// ChangeAncestorsTravelerFn adapts a function to the ChangeAncestorCallback interface.
type ChangeAncestorsTravelerFn func(payload ChangeMeta) bool

func (fn ChangeAncestorsTravelerFn) Travel(payload ChangeMeta) bool {
	return fn(payload)
}

// AsChangeAncestorCallback adapts a function to the ChangeAncestorCallback interface.
func AsChangeAncestorCallback(fn ChangeAncestorsTravelerFn) ChangeAncestorsTraveler {
	return fn
}

// LocalEphemeralListenerFn adapts a function to the LocalEphemeralListener interface.
type LocalEphemeralListenerFn func(update []byte)

func (fn LocalEphemeralListenerFn) OnEphemeralUpdate(update []byte) {
	fn(update)
}

// AsLocalEphemeralListener adapts a function to the LocalEphemeralListener interface.
func AsLocalEphemeralListener(fn LocalEphemeralListenerFn) LocalEphemeralListener {
	return fn
}

// EphemeralSubscriberFn adapts a function to the EphemeralSubscriber interface.
type EphemeralSubscriberFn func(event EphemeralStoreEvent)

func (fn EphemeralSubscriberFn) OnEphemeralEvent(event EphemeralStoreEvent) {
	fn(event)
}

// AsEphemeralSubscriber adapts a function to the EphemeralSubscriber interface.
func AsEphemeralSubscriber(fn EphemeralSubscriberFn) EphemeralSubscriber {
	return fn
}
