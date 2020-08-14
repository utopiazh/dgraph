// Copyright 2020 ChainSafe Systems (ON) Corp.
// This file is part of gossamer.
//
// The gossamer library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The gossamer library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the gossamer library. If not, see <http://www.gnu.org/licenses/>.

package grandpa

import (
	"github.com/ChainSafe/gossamer/lib/scale"
)

// MessageHandler handles GRANDPA consensus messages
type MessageHandler struct {
	grandpa    *Service
	blockState BlockState
}

// NewMessageHandler returns a new MessageHandler
func NewMessageHandler(grandpa *Service, blockState BlockState) *MessageHandler {
	return &MessageHandler{
		grandpa:    grandpa,
		blockState: blockState,
	}
}

// HandleMessage handles a GRANDPA consensus message
// if it is a FinalizationMessage, it updates the BlockState
// if it is a VoteMessage, it sends it to the GRANDPA service
func (h *MessageHandler) HandleMessage(msg *ConsensusMessage) (*ConsensusMessage, error) {
	m, err := decodeMessage(msg)
	if err != nil {
		return nil, err
	}

	fm, ok := m.(*FinalizationMessage)
	if ok {
		return h.handleFinalizationMessage(fm)
	}

	vm, ok := m.(*VoteMessage)
	if h.grandpa != nil && ok {
		// send vote message to grandpa service
		h.grandpa.in <- vm
	}

	return nil, nil
}

func (h *MessageHandler) handleFinalizationMessage(msg *FinalizationMessage) (*ConsensusMessage, error) {
	// check if msg has same setID but is 2 or more rounds ahead of us, if so, return catch-up request to send
	if msg.Round > h.grandpa.state.round+1 { // TODO: FinalizationMessage does not have setID, confirm this is correct
		req := newCatchUpRequest(msg.Round, h.grandpa.state.setID)
		return req.ToConsensusMessage()
	}

	// TODO: check justification here

	// set finalized head for round in db
	err := h.blockState.SetFinalizedHash(msg.Vote.hash, msg.Round, h.grandpa.state.setID)
	if err != nil {
		return nil, err
	}

	// set latest finalized head in db
	err = h.blockState.SetFinalizedHash(msg.Vote.hash, 0, 0)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// decodeMessage decodes a network-level consensus message into a GRANDPA VoteMessage or FinalizationMessage
func decodeMessage(msg *ConsensusMessage) (m FinalityMessage, err error) {
	var mi interface{}

	switch msg.Data[0] {
	case voteType, precommitType:
		mi, err = scale.Decode(msg.Data[1:], &VoteMessage{Message: new(SignedMessage)})
		m = mi.(*VoteMessage)
	case finalizationType:
		mi, err = scale.Decode(msg.Data[1:], &FinalizationMessage{})
		m = mi.(*FinalizationMessage)
	default:
		return nil, ErrInvalidMessageType
	}

	if err != nil {
		return nil, err
	}

	return m, nil
}