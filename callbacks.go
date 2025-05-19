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
