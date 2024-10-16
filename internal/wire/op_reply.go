// Copyright 2021 FerretDB Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wire

import (
	"encoding/binary"
	"encoding/json"

	"github.com/FerretDB/FerretDB/internal/bson2"
	"github.com/FerretDB/FerretDB/internal/types"
	"github.com/FerretDB/FerretDB/internal/types/fjson"
	"github.com/FerretDB/FerretDB/internal/util/debugbuild"
	"github.com/FerretDB/FerretDB/internal/util/lazyerrors"
	"github.com/FerretDB/FerretDB/internal/util/must"
)

// OpReply is a deprecated response message type.
//
// Only up to one returned document is supported.
type OpReply struct {
	ResponseFlags OpReplyFlags
	CursorID      int64
	StartingFrom  int32
	document      bson2.RawDocument
}

func (reply *OpReply) msgbody() {}

// check checks if the reply is valid.
func (reply *OpReply) check() error {
	if !debugbuild.Enabled {
		return nil
	}

	if d := reply.document; d != nil {
		if _, err := d.DecodeDeep(); err != nil {
			return lazyerrors.Error(err)
		}
	}

	return nil
}

// UnmarshalBinaryNocopy implements [MsgBody] interface.
func (reply *OpReply) UnmarshalBinaryNocopy(b []byte) error {
	if len(b) < 20 {
		return lazyerrors.Errorf("len=%d", len(b))
	}

	reply.ResponseFlags = OpReplyFlags(binary.LittleEndian.Uint32(b[0:4]))
	reply.CursorID = int64(binary.LittleEndian.Uint64(b[4:12]))
	reply.StartingFrom = int32(binary.LittleEndian.Uint32(b[12:16]))
	numberReturned := int32(binary.LittleEndian.Uint32(b[16:20]))
	reply.document = b[20:]

	if numberReturned < 0 || numberReturned > 1 {
		return lazyerrors.Errorf("numberReturned=%d", numberReturned)
	}

	if len(reply.document) == 0 {
		reply.document = nil
	}

	if (numberReturned == 0) != (reply.document == nil) {
		return lazyerrors.Errorf("numberReturned=%d, document=%v", numberReturned, reply.document)
	}

	if err := reply.check(); err != nil {
		return lazyerrors.Error(err)
	}

	return nil
}

// MarshalBinary implements [MsgBody] interface.
func (reply *OpReply) MarshalBinary() ([]byte, error) {
	if err := reply.check(); err != nil {
		return nil, lazyerrors.Error(err)
	}

	b := make([]byte, 20+len(reply.document))

	binary.LittleEndian.PutUint32(b[0:4], uint32(reply.ResponseFlags))
	binary.LittleEndian.PutUint64(b[4:12], uint64(reply.CursorID))
	binary.LittleEndian.PutUint32(b[12:16], uint32(reply.StartingFrom))

	if reply.document == nil {
		binary.LittleEndian.PutUint32(b[16:20], uint32(0))
	} else {
		binary.LittleEndian.PutUint32(b[16:20], uint32(1))
		copy(b[20:], reply.document)
	}

	return b, nil
}

// Document returns reply document.
func (reply *OpReply) Document() (*types.Document, error) {
	if reply.document == nil {
		return nil, nil
	}

	return reply.document.Convert()
}

// SetDocument sets reply document.
func (reply *OpReply) SetDocument(doc *types.Document) {
	d := must.NotFail(bson2.ConvertDocument(doc))
	reply.document = must.NotFail(d.Encode())
}

// String returns a string representation for logging.
func (reply *OpReply) String() string {
	if reply == nil {
		return "<nil>"
	}

	m := map[string]any{
		"ResponseFlags": reply.ResponseFlags,
		"CursorID":      reply.CursorID,
		"StartingFrom":  reply.StartingFrom,
	}

	if reply.document == nil {
		m["NumberReturned"] = 0
	} else {
		m["NumberReturned"] = 1

		doc, err := reply.document.Convert()
		if err == nil {
			m["Documents"] = json.RawMessage(must.NotFail(fjson.Marshal(doc)))
		} else {
			m["DocumentError"] = err.Error()
		}
	}

	return string(must.NotFail(json.MarshalIndent(m, "", "  ")))
}

// check interfaces
var (
	_ MsgBody = (*OpReply)(nil)
)
