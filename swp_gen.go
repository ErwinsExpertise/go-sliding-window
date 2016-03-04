package swp

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"time"

	"github.com/nats-io/nats"
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Packet) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "SeqNum":
			{
				var tmp int64
				tmp, err = dc.ReadInt64()
				z.SeqNum = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "AckNum":
			{
				var tmp int64
				tmp, err = dc.ReadInt64()
				z.AckNum = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "AckOnly":
			z.AckOnly, err = dc.ReadBool()
			if err != nil {
				return
			}
		case "Data":
			z.Data, err = dc.ReadBytes(z.Data)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Packet) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "SeqNum"
	err = en.Append(0x84, 0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(int64(z.SeqNum))
	if err != nil {
		return
	}
	// write "AckNum"
	err = en.Append(0xa6, 0x41, 0x63, 0x6b, 0x4e, 0x75, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteInt64(int64(z.AckNum))
	if err != nil {
		return
	}
	// write "AckOnly"
	err = en.Append(0xa7, 0x41, 0x63, 0x6b, 0x4f, 0x6e, 0x6c, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteBool(z.AckOnly)
	if err != nil {
		return
	}
	// write "Data"
	err = en.Append(0xa4, 0x44, 0x61, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.Data)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Packet) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "SeqNum"
	o = append(o, 0x84, 0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, int64(z.SeqNum))
	// string "AckNum"
	o = append(o, 0xa6, 0x41, 0x63, 0x6b, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, int64(z.AckNum))
	// string "AckOnly"
	o = append(o, 0xa7, 0x41, 0x63, 0x6b, 0x4f, 0x6e, 0x6c, 0x79)
	o = msgp.AppendBool(o, z.AckOnly)
	// string "Data"
	o = append(o, 0xa4, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendBytes(o, z.Data)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Packet) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "SeqNum":
			{
				var tmp int64
				tmp, bts, err = msgp.ReadInt64Bytes(bts)
				z.SeqNum = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "AckNum":
			{
				var tmp int64
				tmp, bts, err = msgp.ReadInt64Bytes(bts)
				z.AckNum = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "AckOnly":
			z.AckOnly, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				return
			}
		case "Data":
			z.Data, bts, err = msgp.ReadBytesBytes(bts, z.Data)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *Packet) Msgsize() (s int) {
	s = 1 + 7 + msgp.Int64Size + 7 + msgp.Int64Size + 8 + msgp.BoolSize + 5 + msgp.BytesPrefixSize + len(z.Data)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *RecvState) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "NextFrameExpected":
			{
				var tmp int64
				tmp, err = dc.ReadInt64()
				z.NextFrameExpected = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "Rwindow":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Rwindow) >= int(xsz) {
				z.Rwindow = z.Rwindow[:xsz]
			} else {
				z.Rwindow = make([]RxqSlot, xsz)
			}
			for xvk := range z.Rwindow {
				err = z.Rwindow[xvk].DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "Rsem":
			err = z.Rsem.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "RecvWindowSize":
			z.RecvWindowSize, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "Mut":
			err = z.Mut.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "Timeout":
			err = z.Timeout.DecodeMsg(dc)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *RecvState) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 6
	// write "NextFrameExpected"
	err = en.Append(0x86, 0xb1, 0x4e, 0x65, 0x78, 0x74, 0x46, 0x72, 0x61, 0x6d, 0x65, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x65, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(int64(z.NextFrameExpected))
	if err != nil {
		return
	}
	// write "Rwindow"
	err = en.Append(0xa7, 0x52, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77)
	if err != nil {
		return err
	}
	err = en.WriteArrayHeader(uint32(len(z.Rwindow)))
	if err != nil {
		return
	}
	for xvk := range z.Rwindow {
		err = z.Rwindow[xvk].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Rsem"
	err = en.Append(0xa4, 0x52, 0x73, 0x65, 0x6d)
	if err != nil {
		return err
	}
	err = z.Rsem.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "RecvWindowSize"
	err = en.Append(0xae, 0x52, 0x65, 0x63, 0x76, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.RecvWindowSize)
	if err != nil {
		return
	}
	// write "Mut"
	err = en.Append(0xa3, 0x4d, 0x75, 0x74)
	if err != nil {
		return err
	}
	err = z.Mut.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "Timeout"
	err = en.Append(0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	if err != nil {
		return err
	}
	err = z.Timeout.EncodeMsg(en)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *RecvState) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 6
	// string "NextFrameExpected"
	o = append(o, 0x86, 0xb1, 0x4e, 0x65, 0x78, 0x74, 0x46, 0x72, 0x61, 0x6d, 0x65, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x65, 0x64)
	o = msgp.AppendInt64(o, int64(z.NextFrameExpected))
	// string "Rwindow"
	o = append(o, 0xa7, 0x52, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Rwindow)))
	for xvk := range z.Rwindow {
		o, err = z.Rwindow[xvk].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Rsem"
	o = append(o, 0xa4, 0x52, 0x73, 0x65, 0x6d)
	o, err = z.Rsem.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "RecvWindowSize"
	o = append(o, 0xae, 0x52, 0x65, 0x63, 0x76, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt(o, z.RecvWindowSize)
	// string "Mut"
	o = append(o, 0xa3, 0x4d, 0x75, 0x74)
	o, err = z.Mut.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "Timeout"
	o = append(o, 0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	o, err = z.Timeout.MarshalMsg(o)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *RecvState) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "NextFrameExpected":
			{
				var tmp int64
				tmp, bts, err = msgp.ReadInt64Bytes(bts)
				z.NextFrameExpected = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "Rwindow":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Rwindow) >= int(xsz) {
				z.Rwindow = z.Rwindow[:xsz]
			} else {
				z.Rwindow = make([]RxqSlot, xsz)
			}
			for xvk := range z.Rwindow {
				bts, err = z.Rwindow[xvk].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "Rsem":
			bts, err = z.Rsem.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "RecvWindowSize":
			z.RecvWindowSize, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Mut":
			bts, err = z.Mut.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "Timeout":
			bts, err = z.Timeout.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *RecvState) Msgsize() (s int) {
	s = 1 + 18 + msgp.Int64Size + 8 + msgp.ArrayHeaderSize
	for xvk := range z.Rwindow {
		s += z.Rwindow[xvk].Msgsize()
	}
	s += 5 + z.Rsem.Msgsize() + 15 + msgp.IntSize + 4 + z.Mut.Msgsize() + 8 + z.Timeout.Msgsize()
	return
}

// DecodeMsg implements msgp.Decodable
func (z *RxqSlot) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Valid":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Valid = nil
			} else {
				if z.Valid == nil {
					z.Valid = new(bool)
				}
				*z.Valid, err = dc.ReadBool()
				if err != nil {
					return
				}
			}
		case "Msg":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Msg = nil
			} else {
				if z.Msg == nil {
					z.Msg = new(Packet)
				}
				err = z.Msg.DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *RxqSlot) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Valid"
	err = en.Append(0x82, 0xa5, 0x56, 0x61, 0x6c, 0x69, 0x64)
	if err != nil {
		return err
	}
	if z.Valid == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = en.WriteBool(*z.Valid)
		if err != nil {
			return
		}
	}
	// write "Msg"
	err = en.Append(0xa3, 0x4d, 0x73, 0x67)
	if err != nil {
		return err
	}
	if z.Msg == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = z.Msg.EncodeMsg(en)
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *RxqSlot) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Valid"
	o = append(o, 0x82, 0xa5, 0x56, 0x61, 0x6c, 0x69, 0x64)
	if z.Valid == nil {
		o = msgp.AppendNil(o)
	} else {
		o = msgp.AppendBool(o, *z.Valid)
	}
	// string "Msg"
	o = append(o, 0xa3, 0x4d, 0x73, 0x67)
	if z.Msg == nil {
		o = msgp.AppendNil(o)
	} else {
		o, err = z.Msg.MarshalMsg(o)
		if err != nil {
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *RxqSlot) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Valid":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Valid = nil
			} else {
				if z.Valid == nil {
					z.Valid = new(bool)
				}
				*z.Valid, bts, err = msgp.ReadBoolBytes(bts)
				if err != nil {
					return
				}
			}
		case "Msg":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Msg = nil
			} else {
				if z.Msg == nil {
					z.Msg = new(Packet)
				}
				bts, err = z.Msg.UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *RxqSlot) Msgsize() (s int) {
	s = 1 + 6
	if z.Valid == nil {
		s += msgp.NilSize
	} else {
		s += msgp.BoolSize
	}
	s += 4
	if z.Msg == nil {
		s += msgp.NilSize
	} else {
		s += z.Msg.Msgsize()
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *SWP) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Sender":
			err = z.Sender.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "Recver":
			err = z.Recver.DecodeMsg(dc)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *SWP) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "Sender"
	err = en.Append(0x82, 0xa6, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72)
	if err != nil {
		return err
	}
	err = z.Sender.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "Recver"
	err = en.Append(0xa6, 0x52, 0x65, 0x63, 0x76, 0x65, 0x72)
	if err != nil {
		return err
	}
	err = z.Recver.EncodeMsg(en)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *SWP) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "Sender"
	o = append(o, 0x82, 0xa6, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72)
	o, err = z.Sender.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "Recver"
	o = append(o, 0xa6, 0x52, 0x65, 0x63, 0x76, 0x65, 0x72)
	o, err = z.Recver.MarshalMsg(o)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *SWP) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Sender":
			bts, err = z.Sender.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "Recver":
			bts, err = z.Recver.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *SWP) Msgsize() (s int) {
	s = 1 + 7 + z.Sender.Msgsize() + 7 + z.Recver.Msgsize()
	return
}

// DecodeMsg implements msgp.Decodable
func (z *SenderState) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "LastAckRecvd":
			{
				var tmp int64
				tmp, err = dc.ReadInt64()
				z.LastAckRecvd = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "LastFrameSent":
			{
				var tmp int64
				tmp, err = dc.ReadInt64()
				z.LastFrameSent = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "Swindow":
			var xsz uint32
			xsz, err = dc.ReadArrayHeader()
			if err != nil {
				return
			}
			if cap(z.Swindow) >= int(xsz) {
				z.Swindow = z.Swindow[:xsz]
			} else {
				z.Swindow = make([]TxqSlot, xsz)
			}
			for bzg := range z.Swindow {
				err = z.Swindow[bzg].DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "Ssem":
			err = z.Ssem.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "SenderWindowSize":
			z.SenderWindowSize, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "Mut":
			err = z.Mut.DecodeMsg(dc)
			if err != nil {
				return
			}
		case "Timeout":
			err = z.Timeout.DecodeMsg(dc)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *SenderState) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 7
	// write "LastAckRecvd"
	err = en.Append(0x87, 0xac, 0x4c, 0x61, 0x73, 0x74, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x63, 0x76, 0x64)
	if err != nil {
		return err
	}
	err = en.WriteInt64(int64(z.LastAckRecvd))
	if err != nil {
		return
	}
	// write "LastFrameSent"
	err = en.Append(0xad, 0x4c, 0x61, 0x73, 0x74, 0x46, 0x72, 0x61, 0x6d, 0x65, 0x53, 0x65, 0x6e, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt64(int64(z.LastFrameSent))
	if err != nil {
		return
	}
	// write "Swindow"
	err = en.Append(0xa7, 0x53, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77)
	if err != nil {
		return err
	}
	err = en.WriteArrayHeader(uint32(len(z.Swindow)))
	if err != nil {
		return
	}
	for bzg := range z.Swindow {
		err = z.Swindow[bzg].EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Ssem"
	err = en.Append(0xa4, 0x53, 0x73, 0x65, 0x6d)
	if err != nil {
		return err
	}
	err = z.Ssem.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "SenderWindowSize"
	err = en.Append(0xb0, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.SenderWindowSize)
	if err != nil {
		return
	}
	// write "Mut"
	err = en.Append(0xa3, 0x4d, 0x75, 0x74)
	if err != nil {
		return err
	}
	err = z.Mut.EncodeMsg(en)
	if err != nil {
		return
	}
	// write "Timeout"
	err = en.Append(0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	if err != nil {
		return err
	}
	err = z.Timeout.EncodeMsg(en)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *SenderState) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 7
	// string "LastAckRecvd"
	o = append(o, 0x87, 0xac, 0x4c, 0x61, 0x73, 0x74, 0x41, 0x63, 0x6b, 0x52, 0x65, 0x63, 0x76, 0x64)
	o = msgp.AppendInt64(o, int64(z.LastAckRecvd))
	// string "LastFrameSent"
	o = append(o, 0xad, 0x4c, 0x61, 0x73, 0x74, 0x46, 0x72, 0x61, 0x6d, 0x65, 0x53, 0x65, 0x6e, 0x74)
	o = msgp.AppendInt64(o, int64(z.LastFrameSent))
	// string "Swindow"
	o = append(o, 0xa7, 0x53, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Swindow)))
	for bzg := range z.Swindow {
		o, err = z.Swindow[bzg].MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Ssem"
	o = append(o, 0xa4, 0x53, 0x73, 0x65, 0x6d)
	o, err = z.Ssem.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "SenderWindowSize"
	o = append(o, 0xb0, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt(o, z.SenderWindowSize)
	// string "Mut"
	o = append(o, 0xa3, 0x4d, 0x75, 0x74)
	o, err = z.Mut.MarshalMsg(o)
	if err != nil {
		return
	}
	// string "Timeout"
	o = append(o, 0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	o, err = z.Timeout.MarshalMsg(o)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *SenderState) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "LastAckRecvd":
			{
				var tmp int64
				tmp, bts, err = msgp.ReadInt64Bytes(bts)
				z.LastAckRecvd = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "LastFrameSent":
			{
				var tmp int64
				tmp, bts, err = msgp.ReadInt64Bytes(bts)
				z.LastFrameSent = Seqno(tmp)
			}
			if err != nil {
				return
			}
		case "Swindow":
			var xsz uint32
			xsz, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				return
			}
			if cap(z.Swindow) >= int(xsz) {
				z.Swindow = z.Swindow[:xsz]
			} else {
				z.Swindow = make([]TxqSlot, xsz)
			}
			for bzg := range z.Swindow {
				bts, err = z.Swindow[bzg].UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "Ssem":
			bts, err = z.Ssem.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "SenderWindowSize":
			z.SenderWindowSize, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "Mut":
			bts, err = z.Mut.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		case "Timeout":
			bts, err = z.Timeout.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *SenderState) Msgsize() (s int) {
	s = 1 + 13 + msgp.Int64Size + 14 + msgp.Int64Size + 8 + msgp.ArrayHeaderSize
	for bzg := range z.Swindow {
		s += z.Swindow[bzg].Msgsize()
	}
	s += 5 + z.Ssem.Msgsize() + 17 + msgp.IntSize + 4 + z.Mut.Msgsize() + 8 + z.Timeout.Msgsize()
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Seqno) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var tmp int64
		tmp, err = dc.ReadInt64()
		(*z) = Seqno(tmp)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Seqno) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt64(int64(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Seqno) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt64(o, int64(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Seqno) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var tmp int64
		tmp, bts, err = msgp.ReadInt64Bytes(bts)
		(*z) = Seqno(tmp)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

func (z Seqno) Msgsize() (s int) {
	s = msgp.Int64Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Session) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Swp":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Swp = nil
			} else {
				if z.Swp == nil {
					z.Swp = new(SWP)
				}
				var isz uint32
				isz, err = dc.ReadMapHeader()
				if err != nil {
					return
				}
				for isz > 0 {
					isz--
					field, err = dc.ReadMapKeyPtr()
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Sender":
						err = z.Swp.Sender.DecodeMsg(dc)
						if err != nil {
							return
						}
					case "Recver":
						err = z.Swp.Recver.DecodeMsg(dc)
						if err != nil {
							return
						}
					default:
						err = dc.Skip()
						if err != nil {
							return
						}
					}
				}
			}
		case "Destination":
			z.Destination, err = dc.ReadString()
			if err != nil {
				return
			}
		case "MyInBox":
			z.MyInBox, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Nc":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Nc = nil
			} else {
				if z.Nc == nil {
					z.Nc = new(nats.Conn)
				}
				err = z.Nc.DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Session) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 4
	// write "Swp"
	err = en.Append(0x84, 0xa3, 0x53, 0x77, 0x70)
	if err != nil {
		return err
	}
	if z.Swp == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		// map header, size 2
		// write "Sender"
		err = en.Append(0x82, 0xa6, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72)
		if err != nil {
			return err
		}
		err = z.Swp.Sender.EncodeMsg(en)
		if err != nil {
			return
		}
		// write "Recver"
		err = en.Append(0xa6, 0x52, 0x65, 0x63, 0x76, 0x65, 0x72)
		if err != nil {
			return err
		}
		err = z.Swp.Recver.EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Destination"
	err = en.Append(0xab, 0x44, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Destination)
	if err != nil {
		return
	}
	// write "MyInBox"
	err = en.Append(0xa7, 0x4d, 0x79, 0x49, 0x6e, 0x42, 0x6f, 0x78)
	if err != nil {
		return err
	}
	err = en.WriteString(z.MyInBox)
	if err != nil {
		return
	}
	// write "Nc"
	err = en.Append(0xa2, 0x4e, 0x63)
	if err != nil {
		return err
	}
	if z.Nc == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = z.Nc.EncodeMsg(en)
		if err != nil {
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Session) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 4
	// string "Swp"
	o = append(o, 0x84, 0xa3, 0x53, 0x77, 0x70)
	if z.Swp == nil {
		o = msgp.AppendNil(o)
	} else {
		// map header, size 2
		// string "Sender"
		o = append(o, 0x82, 0xa6, 0x53, 0x65, 0x6e, 0x64, 0x65, 0x72)
		o, err = z.Swp.Sender.MarshalMsg(o)
		if err != nil {
			return
		}
		// string "Recver"
		o = append(o, 0xa6, 0x52, 0x65, 0x63, 0x76, 0x65, 0x72)
		o, err = z.Swp.Recver.MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Destination"
	o = append(o, 0xab, 0x44, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e)
	o = msgp.AppendString(o, z.Destination)
	// string "MyInBox"
	o = append(o, 0xa7, 0x4d, 0x79, 0x49, 0x6e, 0x42, 0x6f, 0x78)
	o = msgp.AppendString(o, z.MyInBox)
	// string "Nc"
	o = append(o, 0xa2, 0x4e, 0x63)
	if z.Nc == nil {
		o = msgp.AppendNil(o)
	} else {
		o, err = z.Nc.MarshalMsg(o)
		if err != nil {
			return
		}
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Session) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Swp":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Swp = nil
			} else {
				if z.Swp == nil {
					z.Swp = new(SWP)
				}
				var isz uint32
				isz, bts, err = msgp.ReadMapHeaderBytes(bts)
				if err != nil {
					return
				}
				for isz > 0 {
					isz--
					field, bts, err = msgp.ReadMapKeyZC(bts)
					if err != nil {
						return
					}
					switch msgp.UnsafeString(field) {
					case "Sender":
						bts, err = z.Swp.Sender.UnmarshalMsg(bts)
						if err != nil {
							return
						}
					case "Recver":
						bts, err = z.Swp.Recver.UnmarshalMsg(bts)
						if err != nil {
							return
						}
					default:
						bts, err = msgp.Skip(bts)
						if err != nil {
							return
						}
					}
				}
			}
		case "Destination":
			z.Destination, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "MyInBox":
			z.MyInBox, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Nc":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Nc = nil
			} else {
				if z.Nc == nil {
					z.Nc = new(nats.Conn)
				}
				bts, err = z.Nc.UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *Session) Msgsize() (s int) {
	s = 1 + 4
	if z.Swp == nil {
		s += msgp.NilSize
	} else {
		s += 1 + 7 + z.Swp.Sender.Msgsize() + 7 + z.Swp.Recver.Msgsize()
	}
	s += 12 + msgp.StringPrefixSize + len(z.Destination) + 8 + msgp.StringPrefixSize + len(z.MyInBox) + 3
	if z.Nc == nil {
		s += msgp.NilSize
	} else {
		s += z.Nc.Msgsize()
	}
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TxqSlot) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Timeout":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Timeout = nil
			} else {
				if z.Timeout == nil {
					z.Timeout = new(time.Timer)
				}
				err = z.Timeout.DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "Session":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					return
				}
				z.Session = nil
			} else {
				if z.Session == nil {
					z.Session = new(Session)
				}
				err = z.Session.DecodeMsg(dc)
				if err != nil {
					return
				}
			}
		case "Msg":
			err = z.Msg.DecodeMsg(dc)
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *TxqSlot) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "Timeout"
	err = en.Append(0x83, 0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	if err != nil {
		return err
	}
	if z.Timeout == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = z.Timeout.EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Session"
	err = en.Append(0xa7, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e)
	if err != nil {
		return err
	}
	if z.Session == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = z.Session.EncodeMsg(en)
		if err != nil {
			return
		}
	}
	// write "Msg"
	err = en.Append(0xa3, 0x4d, 0x73, 0x67)
	if err != nil {
		return err
	}
	err = z.Msg.EncodeMsg(en)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *TxqSlot) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Timeout"
	o = append(o, 0x83, 0xa7, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74)
	if z.Timeout == nil {
		o = msgp.AppendNil(o)
	} else {
		o, err = z.Timeout.MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Session"
	o = append(o, 0xa7, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e)
	if z.Session == nil {
		o = msgp.AppendNil(o)
	} else {
		o, err = z.Session.MarshalMsg(o)
		if err != nil {
			return
		}
	}
	// string "Msg"
	o = append(o, 0xa3, 0x4d, 0x73, 0x67)
	o, err = z.Msg.MarshalMsg(o)
	if err != nil {
		return
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TxqSlot) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var isz uint32
	isz, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for isz > 0 {
		isz--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "Timeout":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Timeout = nil
			} else {
				if z.Timeout == nil {
					z.Timeout = new(time.Timer)
				}
				bts, err = z.Timeout.UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "Session":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Session = nil
			} else {
				if z.Session == nil {
					z.Session = new(Session)
				}
				bts, err = z.Session.UnmarshalMsg(bts)
				if err != nil {
					return
				}
			}
		case "Msg":
			bts, err = z.Msg.UnmarshalMsg(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

func (z *TxqSlot) Msgsize() (s int) {
	s = 1 + 8
	if z.Timeout == nil {
		s += msgp.NilSize
	} else {
		s += z.Timeout.Msgsize()
	}
	s += 8
	if z.Session == nil {
		s += msgp.NilSize
	} else {
		s += z.Session.Msgsize()
	}
	s += 4 + z.Msg.Msgsize()
	return
}
