package rdb

import (
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/mhughdo/src/pkg/crc64"
	"github.com/mhughdo/src/pkg/keyval"
)

type RDBSaver struct {
	wr   io.Writer
	data map[string]keyval.Value
	hash hash.Hash64
}

func NewRDBSaver(data map[string]keyval.Value) *RDBSaver {
	return &RDBSaver{
		data: data,
		hash: crc64.New(),
	}
}

func (s *RDBSaver) write(b []byte) error {
	_, err := s.wr.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func (s *RDBSaver) writeByte(b byte) error {
	return s.write([]byte{b})
}

func (s *RDBSaver) writeBytes(b []byte) error {
	err := s.writeLength(uint64(len(b)))
	if err != nil {
		return err
	}
	return s.write(b)
}

func (s *RDBSaver) writeString(str string) error {
	return s.writeBytes([]byte(str))
}

func (s *RDBSaver) writeLength(length uint64) error {
	if length <= 0x3F {
		// 6 bits (00xxxxxx)
		return s.writeByte(byte(length))
	} else if length <= 0x3FFF {
		// 14 bits (01xxxxxx xxxxxxxx)
		b1 := byte(((length >> 8) & 0xFF) | 0x40)
		b2 := byte(length & 0xFF)
		return s.write([]byte{b1, b2})
	} else {
		// 32 bits (10xxxxxx xxxxxxxx xxxxxxxx xxxxxxxx xxxxxxxx)
		err := s.writeByte(0x80)
		if err != nil {
			return err
		}
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(length))
		return s.write(buf[:])
	}
}

func (s *RDBSaver) writeHeader() error {
	header := fmt.Sprintf("REDIS%04d", 12)
	return s.write([]byte(header))
}

func (s *RDBSaver) writeWithSpecialFormat(val any) error {
	// first 2 bits represent the encoding type, for special format, it's always 11
	// next 6 bits represent the format type, 0 for int8, 1 for int16, 2 for int32, 3 for compressed string
	var b byte = 0xC0
	var buf []byte
	switch v := val.(type) {
	case int8:
		b |= 0
		buf = []byte{byte(v)}
	case int16:
		b |= 1
		buf = make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(v))
	case int32:
		b |= 2
		buf = make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(v))
	default:
		return fmt.Errorf("writeWithSpecialFormat: unsupported value type: %T", val)
	}
	if err := s.writeByte(b); err != nil {
		return err
	}

	return s.write(buf)
}
func (s *RDBSaver) writeAuxiliaryField(key string, value interface{}) error {
	if err := s.writeByte(RDBMarkerAuxiliaryField); err != nil {
		return err
	}

	if err := s.writeString(key); err != nil {
		return err
	}
	switch v := value.(type) {
	case string:
		if err := s.writeString(key); err != nil {
			return err
		}
	default:
		return s.writeWithSpecialFormat(v)
	}

	return nil
}

func (s *RDBSaver) writeExpire(expiry uint64) error {
	if expiry == 0 {
		return nil
	}
	if err := s.writeByte(RDBMarkerExpiryMS); err != nil {
		return err
	}
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], expiry)
	return s.write(buf[:])
}

func (s *RDBSaver) writeObject(key string, val keyval.Value) error {
	// Currently, we only support strings
	if val.Type != keyval.ValueTypeString {
		return fmt.Errorf("unsupported value type: %v", val.Type)
	}
	if val.Expiry > 0 {
		if err := s.writeExpire(val.Expiry); err != nil {
			return err
		}
	}
	if err := s.writeByte(RDBTypeString); err != nil {
		return err
	}
	if err := s.writeString(key); err != nil {
		return err
	}

	data, ok := val.Data.([]byte)
	if !ok {
		return fmt.Errorf("invalid data type for key %s", key)
	}
	return s.writeBytes(data)
}

func (s *RDBSaver) writeChecksum() error {
	if s.hash == nil {
		// If checksum is disabled, write zeros (8 bytes)
		zero := make([]byte, 8)
		return s.write(zero)
	}
	checksum := s.hash.Sum64()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], checksum)
	return s.write(buf[:])
}

func (s *RDBSaver) SaveRDB(wr io.Writer) error {
	s.hash = crc64.New()
	multiWriter := io.MultiWriter(wr, s.hash)
	s.wr = multiWriter

	if err := s.writeHeader(); err != nil {
		return err
	}

	// Write auxiliary fields
	if err := s.writeAuxiliaryField("redis-ver", "7.4.0"); err != nil {
		return err
	}
	if err := s.writeAuxiliaryField("redis-bits", int8(64)); err != nil {
		return err
	}
	ctime := time.Now().Unix()
	if err := s.writeAuxiliaryField("ctime", int32(ctime)); err != nil {
		return err
	}
	if err := s.writeAuxiliaryField("used-mem", int32(0)); err != nil {
		return err
	}
	if err := s.writeAuxiliaryField("aof-base", int8(0)); err != nil {
		return err
	}

	// Select DB 0
	if err := s.writeByte(RDBMarkerSelectDB); err != nil {
		return err
	}
	if err := s.writeLength(0); err != nil {
		return err
	}

	if err := s.writeByte(RDBMarkerResizeDB); err != nil {
		return err
	}
	expirySize := 0
	for _, val := range s.data {
		if val.Expiry > 0 {
			expirySize++
		}
	}
	if err := s.writeLength(uint64(len(s.data))); err != nil {
		return err
	}
	if err := s.writeLength(uint64(expirySize)); err != nil {
		return err
	}

	// Write key-value pairs
	for key, val := range s.data {
		if err := s.writeObject(key, val); err != nil {
			return err
		}
	}

	// Write EOF marker
	if err := s.writeByte(RDBMarkerEOF); err != nil {
		return err
	}

	// Write checksum
	if err := s.writeChecksum(); err != nil {
		return err
	}

	return nil
}
