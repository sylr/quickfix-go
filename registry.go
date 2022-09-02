package quickfix

import (
	"errors"
	"sync"
)

var sessionsLock sync.RWMutex
var sessions = make(map[SessionID]*session)
var errDuplicateSessionID = errors.New("Duplicate SessionID")
var errUnknownSession = errors.New("Unknown session")

// Messagable is a Message or something that can be converted to a Message
type Messagable interface {
	ToMessage() *Message
}

// Send determines the session to send Messagable using header fields BeginString, TargetCompID, SenderCompID
func Send(m Messagable) (err error) {
	msg := m.ToMessage()
	var beginString FIXString
	if err := msg.Header.GetField(tagBeginString, &beginString); err != nil {
		return err
	}

	var targetCompID FIXString
	if err := msg.Header.GetField(tagTargetCompID, &targetCompID); err != nil {
		return err
	}

	var targetSubID FIXString
	msg.Header.GetField(tagTargetSubID, &targetSubID)

	var senderCompID FIXString
	if err := msg.Header.GetField(tagSenderCompID, &senderCompID); err != nil {
		return nil
	}

	var senderSubID FIXString
	msg.Header.GetField(tagSenderSubID, &senderSubID)

	sessionID := SessionID{
		BeginString:  string(beginString),
		TargetCompID: string(targetCompID),
		TargetSubID:  string(targetSubID),
		SenderCompID: string(senderCompID),
		SenderSubID:  string(senderSubID),
	}

	return SendToTarget(msg, sessionID)
}

// SendToTarget sends a message based on the sessionID. Convenient for use in FromApp since it provides a session ID for incoming messages
func SendToTarget(m Messagable, sessionID SessionID) error {
	msg := m.ToMessage()
	session, ok := lookupSession(sessionID)
	if !ok {
		return errUnknownSession
	}

	return session.queueForSend(msg)
}

// SendReject is a helper function which allows to send an error outside of the
// quickfix application methods. Useful when doing asynchronous work.
func SendReject(m *Message, sessionID SessionID, rej MessageRejectError) error {
	session, ok := lookupSession(sessionID)
	if !ok {
		return errUnknownSession
	}

	return session.doReject(m, rej)
}

// UnregisterSession removes a session from the set of known sessions
func UnregisterSession(sessionID SessionID) error {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	if _, ok := sessions[sessionID]; ok {
		delete(sessions, sessionID)
		return nil
	}

	return errUnknownSession
}

func registerSession(s *session) error {
	sessionsLock.Lock()
	defer sessionsLock.Unlock()

	if _, ok := sessions[s.sessionID]; ok {
		return errDuplicateSessionID
	}

	sessions[s.sessionID] = s
	return nil
}

func lookupSession(sessionID SessionID) (s *session, ok bool) {
	sessionsLock.RLock()
	defer sessionsLock.RUnlock()

	s, ok = sessions[sessionID]
	return
}
