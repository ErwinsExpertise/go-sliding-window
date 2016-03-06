package swp

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
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
		case "From":
			z.From, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Dest":
			z.Dest, err = dc.ReadString()
			if err != nil {
				return
			}
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
		case "KeepAlive":
			z.KeepAlive, err = dc.ReadBool()
			if err != nil {
				return
			}
		case "AvailReaderBytesCap":
			z.AvailReaderBytesCap, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "AvailReaderMsgCap":
			z.AvailReaderMsgCap, err = dc.ReadInt64()
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
	// map header, size 9
	// write "From"
	err = en.Append(0x89, 0xa4, 0x46, 0x72, 0x6f, 0x6d)
	if err != nil {
		return err
	}
	err = en.WriteString(z.From)
	if err != nil {
		return
	}
	// write "Dest"
	err = en.Append(0xa4, 0x44, 0x65, 0x73, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Dest)
	if err != nil {
		return
	}
	// write "SeqNum"
	err = en.Append(0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
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
	// write "KeepAlive"
	err = en.Append(0xa9, 0x4b, 0x65, 0x65, 0x70, 0x41, 0x6c, 0x69, 0x76, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteBool(z.KeepAlive)
	if err != nil {
		return
	}
	// write "AvailReaderBytesCap"
	err = en.Append(0xb3, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42, 0x79, 0x74, 0x65, 0x73, 0x43, 0x61, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AvailReaderBytesCap)
	if err != nil {
		return
	}
	// write "AvailReaderMsgCap"
	err = en.Append(0xb1, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x73, 0x67, 0x43, 0x61, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.AvailReaderMsgCap)
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
	// map header, size 9
	// string "From"
	o = append(o, 0x89, 0xa4, 0x46, 0x72, 0x6f, 0x6d)
	o = msgp.AppendString(o, z.From)
	// string "Dest"
	o = append(o, 0xa4, 0x44, 0x65, 0x73, 0x74)
	o = msgp.AppendString(o, z.Dest)
	// string "SeqNum"
	o = append(o, 0xa6, 0x53, 0x65, 0x71, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, int64(z.SeqNum))
	// string "AckNum"
	o = append(o, 0xa6, 0x41, 0x63, 0x6b, 0x4e, 0x75, 0x6d)
	o = msgp.AppendInt64(o, int64(z.AckNum))
	// string "AckOnly"
	o = append(o, 0xa7, 0x41, 0x63, 0x6b, 0x4f, 0x6e, 0x6c, 0x79)
	o = msgp.AppendBool(o, z.AckOnly)
	// string "KeepAlive"
	o = append(o, 0xa9, 0x4b, 0x65, 0x65, 0x70, 0x41, 0x6c, 0x69, 0x76, 0x65)
	o = msgp.AppendBool(o, z.KeepAlive)
	// string "AvailReaderBytesCap"
	o = append(o, 0xb3, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x42, 0x79, 0x74, 0x65, 0x73, 0x43, 0x61, 0x70)
	o = msgp.AppendInt64(o, z.AvailReaderBytesCap)
	// string "AvailReaderMsgCap"
	o = append(o, 0xb1, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x73, 0x67, 0x43, 0x61, 0x70)
	o = msgp.AppendInt64(o, z.AvailReaderMsgCap)
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
		case "From":
			z.From, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Dest":
			z.Dest, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
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
		case "KeepAlive":
			z.KeepAlive, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				return
			}
		case "AvailReaderBytesCap":
			z.AvailReaderBytesCap, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "AvailReaderMsgCap":
			z.AvailReaderMsgCap, bts, err = msgp.ReadInt64Bytes(bts)
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
	s = 1 + 5 + msgp.StringPrefixSize + len(z.From) + 5 + msgp.StringPrefixSize + len(z.Dest) + 7 + msgp.Int64Size + 7 + msgp.Int64Size + 8 + msgp.BoolSize + 10 + msgp.BoolSize + 20 + msgp.Int64Size + 18 + msgp.Int64Size + 5 + msgp.BytesPrefixSize + len(z.Data)
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
func (z *Sum) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "ObsKeepRateFromA":
			z.ObsKeepRateFromA, err = dc.ReadFloat64()
			if err != nil {
				return
			}
		case "ObsKeepRateFromB":
			z.ObsKeepRateFromB, err = dc.ReadFloat64()
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
func (z Sum) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "ObsKeepRateFromA"
	err = en.Append(0x82, 0xb0, 0x4f, 0x62, 0x73, 0x4b, 0x65, 0x65, 0x70, 0x52, 0x61, 0x74, 0x65, 0x46, 0x72, 0x6f, 0x6d, 0x41)
	if err != nil {
		return err
	}
	err = en.WriteFloat64(z.ObsKeepRateFromA)
	if err != nil {
		return
	}
	// write "ObsKeepRateFromB"
	err = en.Append(0xb0, 0x4f, 0x62, 0x73, 0x4b, 0x65, 0x65, 0x70, 0x52, 0x61, 0x74, 0x65, 0x46, 0x72, 0x6f, 0x6d, 0x42)
	if err != nil {
		return err
	}
	err = en.WriteFloat64(z.ObsKeepRateFromB)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Sum) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "ObsKeepRateFromA"
	o = append(o, 0x82, 0xb0, 0x4f, 0x62, 0x73, 0x4b, 0x65, 0x65, 0x70, 0x52, 0x61, 0x74, 0x65, 0x46, 0x72, 0x6f, 0x6d, 0x41)
	o = msgp.AppendFloat64(o, z.ObsKeepRateFromA)
	// string "ObsKeepRateFromB"
	o = append(o, 0xb0, 0x4f, 0x62, 0x73, 0x4b, 0x65, 0x65, 0x70, 0x52, 0x61, 0x74, 0x65, 0x46, 0x72, 0x6f, 0x6d, 0x42)
	o = msgp.AppendFloat64(o, z.ObsKeepRateFromB)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Sum) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "ObsKeepRateFromA":
			z.ObsKeepRateFromA, bts, err = msgp.ReadFloat64Bytes(bts)
			if err != nil {
				return
			}
		case "ObsKeepRateFromB":
			z.ObsKeepRateFromB, bts, err = msgp.ReadFloat64Bytes(bts)
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

func (z Sum) Msgsize() (s int) {
	s = 1 + 17 + msgp.Float64Size + 17 + msgp.Float64Size
	return
}
