package rdb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	lzf "github.com/zhuyie/golzf"

	"github.com/mhughdo/src/pkg/crc64"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/telemetry/logger"
)

const (
	RDBTypeString             = 0
	RDBTypeList               = 1
	RDBTypeSet                = 2
	RDBTypeSortedSet          = 3
	RDBTypeHash               = 4
	RDBTypeZipMap             = 9
	RDBTypeZipList            = 10
	RDBTypeIntSet             = 11
	RDBTypeSortedSetInZipList = 12
	RDBTypeHashMapInZipList   = 13
	RDBTypeListInQuickList    = 14

	RDBMarkerAuxiliaryField = 0xFA // FA: Auxiliary field
	RDBMarkerResizeDB       = 0xFB // FB: Resize DB
	RDBMarkerExpiryMS       = 0xFC // FC: Expire time in milliseconds
	RDBMarkerExpiry         = 0xFD // FD: Expire time in seconds
	RDBMarkerSelectDB       = 0xFE // FE: Select database
	RDBMarkerEOF            = 0xFF // FF: End of the RDB file
)

var ErrInvalidRDBHeader = errors.New("invalid RDB header")

type RDBParser struct {
	rd     io.ReadSeeker
	length uint64
	db     uint64
	data   map[string]keyval.Value
}

func NewRDBParser(rd io.ReadSeeker) *RDBParser {
	return &RDBParser{
		rd:   rd,
		data: make(map[string]keyval.Value),
	}
}

func (p *RDBParser) readByte() (byte, error) {
	var b [1]byte
	_, err := io.ReadFull(p.rd, b[:])
	return b[0], err
}

func (p *RDBParser) readBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(p.rd, b)
	return b, err
}

func (p *RDBParser) readLength() (length uint64, special bool, err error) {
	b, err := p.readByte()
	if err != nil {
		return 0, false, fmt.Errorf("failed to read length: %w", err)
	}

	encType := (b & 0xC0) >> 6
	if encType == 0 {
		return uint64(b & 0x3F), false, nil
	} else if encType == 1 {
		nextByte, err := p.readByte()
		if err != nil {
			return 0, false, fmt.Errorf("failed to read next byte: %w", err)
		}
		return uint64(b&0x3F)<<8 | uint64(nextByte), false, nil
	} else if encType == 2 {
		bb, err := p.readBytes(4)
		if err != nil {
			return 0, false, fmt.Errorf("failed to read 4 bytes: %w", err)
		}
		return uint64(binary.BigEndian.Uint32(bb)), false, nil
	} else if encType == 3 { // 00000011
		// The next object is encoded in a special format. The remaining 6 bits indicate the format.
		return uint64(b & 0x3F), true, nil
	}
	return 0, false, fmt.Errorf("invalid length encoding: %d", encType)
}

func (p *RDBParser) readString() (string, error) {
	length, special, err := p.readLength()
	if err != nil {
		return "", err
	}
	if special {
		// 0 indicates that an 8 bit integer follows
		// 1 indicates that a 16 bit integer follows
		// 2 indicates that a 32 bit integer follows
		switch length {
		case 0:
			b, err := p.readByte()
			if err != nil {
				return "", fmt.Errorf("failed to read 8 bit integer: %w", err)
			}
			return fmt.Sprintf("%d", b), nil
		case 1:
			bb, err := p.readBytes(2)
			if err != nil {
				return "", fmt.Errorf("failed to read 16 bit integer: %w", err)
			}
			return fmt.Sprintf("%d", binary.LittleEndian.Uint16(bb)), nil

		case 2:
			bb, err := p.readBytes(4)
			if err != nil {
				return "", fmt.Errorf("failed to read 32 bit integer: %w", err)
			}
			return fmt.Sprintf("%d", binary.LittleEndian.Uint32(bb)), nil
		case 3:
			//  If the value of those 6 bits is 3, it indicates that a compressed string follows.
			// The compressed string is read as follows:
			// The compressed length clen is read from the stream using Length Encoding
			// The uncompressed length is read from the stream using Length Encoding
			// The next clen bytes are read from the stream
			// Finally, these bytes are decompressed using LZF algorithm
			compressedLength, _, err := p.readLength()
			if err != nil {
				return "", fmt.Errorf("failed to read compressed length: %w", err)
			}
			_, _, err = p.readLength()
			if err != nil {
				return "", fmt.Errorf("failed to read uncompressed length: %w", err)
			}
			compressed, err := p.readBytes(int(compressedLength))
			if err != nil {
				return "", fmt.Errorf("failed to read compressed data: %w", err)
			}
			decompressed := make([]byte, int(compressedLength))
			_, err = lzf.Decompress(compressed, decompressed)
			if err != nil {
				return "", fmt.Errorf("failed to decompress data: %w", err)
			}
			return string(decompressed), nil
		default:
			return "", fmt.Errorf("unsupported special string encoding: %d", length)
		}
	}
	b, err := p.readBytes(int(length))
	if err != nil {
		return "", fmt.Errorf("failed to read string: %w", err)
	}
	return string(b), nil
}

func (p *RDBParser) readValueAsBytes() ([]byte, error) {
	str, err := p.readString()
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

func (p *RDBParser) readHeader() error {
	header := make([]byte, 9)
	if _, err := io.ReadFull(p.rd, header); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}
	if !bytes.Equal(header[:5], []byte("REDIS")) {
		return ErrInvalidRDBHeader
	}
	return nil
}

func (p *RDBParser) handlerAuxiliaryField(ctx context.Context) error {
	key, err := p.readString()
	if err != nil {
		return err
	}
	value, err := p.readString()
	if err != nil {
		return err
	}
	switch key {
	case "redis-ver":
		logger.Info(ctx, "Loading RDB produced by version %s", value)
	case "ctime":
		ctime, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse ctime: %w", err)
		}
		logger.Info(ctx, "RDB age %d seconds", time.Now().Unix()-ctime)
	case "used-mem":
		usedMem, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse used-mem: %w", err)
		}
		logger.Info(ctx, "RDB memory usage when created %.2f MB", float64(usedMem)/1024/1024)
	case "redis-bits":
		redisBits, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse redis-bits: %w", err)
		}
		logger.Info(ctx, "RDB is %d bit", redisBits)
	case "aof-base":
		logger.Info(ctx, "AOF enabled: %s", value)
	}

	return nil
}

func (p *RDBParser) handleSelectDB() error {
	db, _, err := p.readLength()
	if err != nil {
		return err
	}
	p.db = db
	return nil
}

func (p *RDBParser) handleResizeDB() error {
	length, _, err := p.readLength()
	if err != nil {
		return err
	}
	p.length = length
	_, _, err = p.readLength()
	if err != nil {
		return err
	}
	return nil
}

func (p *RDBParser) handleExpiryTime(ms bool) error {
	var expiry uint64

	if ms {
		expBytes, err := p.readBytes(8)
		if err != nil {
			return err
		}
		expiry = binary.LittleEndian.Uint64(expBytes)
	} else {
		expBytes, err := p.readBytes(4)
		if err != nil {
			return err
		}
		expiry = uint64(binary.LittleEndian.Uint32(expBytes))
	}
	objType, err := p.readByte()
	if err != nil {
		return err
	}

	return p.handleObject(objType, expiry)
}

func (p *RDBParser) handleObject(objType byte, expiry uint64) error {
	if objType != RDBTypeString {
		return fmt.Errorf("unsupported object type: %s", objTypeName(objType))
	}
	key, err := p.readString()
	if err != nil {
		return err
	}
	bytes, err := p.readValueAsBytes()
	if err != nil {
		return err
	}
	if expiry > 0 && expiry < uint64(time.Now().UnixMilli()) {
		return nil
	}
	p.data[key] = keyval.Value{
		Type:   keyval.ValueTypeString,
		Data:   bytes,
		Expiry: expiry,
	}
	return nil
}

func objTypeName(objType byte) string {
	switch objType {
	case RDBTypeString:
		return "string"
	case RDBTypeList:
		return "list"
	case RDBTypeSet:
		return "set"
	case RDBTypeSortedSet:
		return "sorted set"
	case RDBTypeHash:
		return "hash"
	case RDBTypeZipMap:
		return "zip map"
	case RDBTypeZipList:
		return "zip list"
	case RDBTypeIntSet:
		return "int set"
	case RDBTypeSortedSetInZipList:
		return "sorted set in zip list"
	case RDBTypeHashMapInZipList:
		return "hash map in zip list"
	case RDBTypeListInQuickList:
		return "list in quick list"
	default:
		return "unknown"
	}
}

func (p *RDBParser) readFileContent() ([]byte, error) {
	currentOffset, err := p.rd.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// Go to the start of the file
	if _, err := p.rd.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	content, err := io.ReadAll(p.rd)
	if err != nil {
		return nil, err
	}

	// Restore reader to current offset
	if _, err := p.rd.Seek(currentOffset, io.SeekStart); err != nil {
		return nil, err
	}

	return content, nil
}

func (p *RDBParser) verifyChecksum() error {
	content, err := p.readFileContent()
	if err != nil {
		return err
	}

	// Exclude the last 8 bytes (checksum)
	if len(content) < 8 {
		return errors.New("RDB file too short")
	}
	data := content[:len(content)-8]
	expectedChecksum := binary.LittleEndian.Uint64(content[len(content)-8:])

	crc64Checksum := crc64.New()
	_, err = crc64Checksum.Write(data)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	if calculatedChecksum := crc64Checksum.Sum64(); calculatedChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %v, got %v", expectedChecksum, calculatedChecksum)
	}
	return nil
}

func (p *RDBParser) ParseRDB(ctx context.Context) error {
	if err := p.readHeader(); err != nil {
		return err
	}
fileLoop:
	for {
		objType, err := p.readByte()
		if err == io.EOF {
			logger.Info(ctx, "Parsing RDB file completed")
			break
		}
		if err != nil {
			return err
		}
		switch objType {
		case RDBMarkerEOF:
			err := p.verifyChecksum()
			if err != nil {
				return fmt.Errorf("failed to verify checksum: %w", err)
			}
			break fileLoop
		case RDBMarkerAuxiliaryField:
			if err := p.handlerAuxiliaryField(ctx); err != nil {
				return fmt.Errorf("failed to handle auxiliary field: %w", err)
			}
		case RDBMarkerSelectDB:
			if err := p.handleSelectDB(); err != nil {
				return fmt.Errorf("failed to handle select db: %w", err)
			}
		case RDBMarkerResizeDB:
			if err := p.handleResizeDB(); err != nil {
				return fmt.Errorf("failed to handle resize db: %w", err)
			}
		case RDBMarkerExpiry:
			if err := p.handleExpiryTime(false); err != nil {
				return fmt.Errorf("failed to handle expiry time: %w", err)
			}
		case RDBMarkerExpiryMS:
			if err := p.handleExpiryTime(true); err != nil {
				return fmt.Errorf("failed to handle expiry time: %w", err)
			}
		default:
			if err := p.handleObject(objType, 0); err != nil {
				return fmt.Errorf("failed to handle object: %w", err)
			}
		}
	}
	logger.Info(ctx, "Done loading RDB, keys loaded: %d, keys expired: %d", len(p.data), p.length-uint64(len(p.data)))
	return nil
}

func (r *RDBParser) GetData() map[string]keyval.Value {
	return r.data
}
