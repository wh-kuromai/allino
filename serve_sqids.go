package allino

import "github.com/sqids/sqids-go"

type SqidsConfig struct {
	Alphabet  string   `json:"alphabet"`
	MinLength uint8    `json:"minlength"`
	Blocklist []string `json:"blocklist"`
}

func (c *SqidsConfig) setup() (*Sqids, error) {
	sq, err := sqids.New(sqids.Options{
		Alphabet:  c.Alphabet,
		MinLength: c.MinLength,
		Blocklist: c.Blocklist,
	})
	if err != nil {
		return nil, err
	}
	return &Sqids{sq}, nil
}

func (r *Request) Sqids() *Sqids {
	return r.server.Sqids
}

type Sqids struct {
	*sqids.Sqids
}

func (s *Sqids) EncodeN(n uint64) string {
	sq, err := s.Sqids.Encode([]uint64{n})
	if err != nil {
		return ""
	}
	return sq
}

func (s *Sqids) DecodeN(n string) uint64 {
	sq := s.Sqids.Decode(n)
	if len(sq) == 0 {
		return 0
	}
	return sq[0]
}
