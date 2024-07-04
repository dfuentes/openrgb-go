package openrgb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

// Client is a TCP client that connects to the OpenRGB Server.
type Client struct {
	clientSock net.Conn
}

// Close the underlying TCP socket.
func (c *Client) Close() error {
	return c.clientSock.Close()
}

// Connect takes in the host and port of the OpenRGB server and creates a TCP socket.
// Returns an instance of `*openrgb.Client` or an error.
func Connect(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	sock, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{clientSock: sock}

	err = c.sendMessage(commandSetClientName, 0, bytes.NewBufferString("GoClient"))
	if err != nil {
		return nil, err
	}

	return c, nil
}

// GetGetControllerCount returns the total number of devices detected by OpenRGB.
// The controller count starts from 0, which means, for `n` number of controllers,
// the count will be `n-1`.
func (c *Client) GetControllerCount() (int, error) {
	err := c.sendMessage(commandRequestControllerCount, 0, nil)
	if err != nil {
		return 0, err
	}

	message, err := c.readMessage()
	if err != nil {
		return 0, err
	}
	count := int(binary.LittleEndian.Uint32(message))

	return count, nil
}

// GetDeviceController queries the OpenRGB server for a device and returns its `openrgb.Device`
// representation. The `deviceID` parameter is an index that starts from 0.
func (c *Client) GetDeviceController(deviceID int) (Device, error) {
	if err := c.sendMessage(commandRequestControllerData, deviceID, nil); err != nil {
		return Device{}, err
	}
	message, err := c.readMessage()
	if err != nil {
		return Device{}, err
	}

	d, err := readDevice(message)
	if err != nil {
		return Device{}, err
	}

	return d, nil
}

// UpdateLEDs updates multiple LEDs on device-level. Length of the `colors` parameter
// MUST match the length of `openrgb.Device.Colors`.
func (c *Client) UpdateLEDs(deviceID int, colors []Color) error {
	lenColors := len(colors)
	size := 2 + (4 * lenColors)

	colorsBuffer := make([]byte, size)
	colorsBuffer[0] = byte(lenColors)

	for i, color := range colors {
		offset := 2 + (i * 4)

		colorsBuffer[offset] = color.Red
		colorsBuffer[offset+1] = color.Green
		colorsBuffer[offset+2] = color.Blue
	}

	prefixBuffer := make([]byte, 4)
	prefixBuffer[0] = byte(size)

	cmd := bytes.NewBuffer(prefixBuffer)
	if _, err := cmd.Write(colorsBuffer); err != nil {
		return err
	}

	return c.sendMessage(commandUpdateLEDs, deviceID, cmd)
}

// UpdateZoneLEDs updates multiple LEDs on zone-level. Length of the `colors` parameter
// MUST match the length of `Colors` parameter in `openrgb.Zone`
func (c *Client) UpdateZoneLEDs(deviceID, zoneID int, colors []Color) error {
	lenColors := len(colors)
	size := 6 + (4 * lenColors)

	colorsBuffer := make([]byte, size)
	colorsBuffer[0] = byte(zoneID)
	colorsBuffer[offset32LEBits] = byte(lenColors)

	for i, color := range colors {
		offset := 6 + (i * 4)

		colorsBuffer[offset] = color.Red
		colorsBuffer[offset+1] = color.Green
		colorsBuffer[offset+2] = color.Blue
	}

	prefixBuffer := make([]byte, 4)
	prefixBuffer[0] = byte(size)

	cmd := bytes.NewBuffer(prefixBuffer)
	if _, err := cmd.Write(colorsBuffer); err != nil {
		return err
	}

	return c.sendMessage(commandUpdateZoneLEDs, deviceID, cmd)
}

func (c *Client) sendMessage(command, deviceID int, buffer *bytes.Buffer) error {
	bufLen := 0
	if buffer != nil {
		bufLen = buffer.Len()
	}

	header := encodeHeader(orgbHeader{
		deviceID:  uint32(deviceID),
		commandID: uint32(command),
		length:    uint32(bufLen),
	})

	if buffer != nil {
		header.Write(buffer.Bytes())
	}

	_, err := c.clientSock.Write(header.Bytes())

	return err
}

func (c *Client) readMessage() ([]byte, error) {
	buf := make([]byte, 16)
	_, err := c.clientSock.Read(buf)
	if err != nil {
		return nil, err
	}

	header := decodeHeader(buf)
	buf = make([]byte, header.length)
	_, err = c.clientSock.Read(buf)

	return buf, err
}
