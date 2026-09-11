package checksum

const mask32 uint32 = 0xffffffff

// Legacy is the byte-at-a-time executable CRC accumulator.
type Legacy struct{ value uint32 }

func (c *Legacy) Add(data []byte) {
	for _, b := range data {
		hibit := c.value >> 31
		c.value = ((c.value << 1) + uint32(b) + hibit) & mask32
	}
}

func (c *Legacy) Value() uint32 { return c.value }

// Xfer is the four-byte INI CRC accumulator.
type Xfer struct{ value uint32 }

func (c *Xfer) addBE(value uint32) {
	hibit := c.value >> 31
	c.value = ((c.value << 1) + value + hibit) & mask32
}

func (c *Xfer) Add(data []byte) {
	for len(data) >= 4 {
		c.addBE(uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]))
		data = data[4:]
	}
	if len(data) == 0 {
		return
	}
	value := uint32(data[0])
	if len(data) >= 2 {
		value |= uint32(data[1]) << 8
	}
	if len(data) == 3 {
		value |= uint32(data[2]) << 16
	}
	hibit := c.value >> 31
	c.value = ((c.value << 1) + value + hibit) & mask32
}

func (c *Xfer) Value() uint32 {
	v := c.value
	return v>>24 | (v>>8)&0xff00 | (v<<8)&0xff0000 | v<<24
}

// INILines applies the engine's comment and control-character normalization.
func INILines(data []byte, visit func([]byte)) {
	for {
		line := data
		if index := indexByte(data, '\n'); index >= 0 {
			line, data = data[:index], data[index+1:]
		} else {
			data = nil
		}
		if index := indexByte(line, ';'); index >= 0 {
			line = line[:index]
		}
		normalized := append([]byte(nil), line...)
		for index, b := range normalized {
			if b > 0 && b < 32 {
				normalized[index] = ' '
			}
		}
		visit(normalized)
		if data == nil {
			return
		}
	}
}

func indexByte(data []byte, needle byte) int {
	for index, b := range data {
		if b == needle {
			return index
		}
	}
	return -1
}
