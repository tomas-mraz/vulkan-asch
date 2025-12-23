// Copyright (c) 2025 Cubyte.online under the AGPL License
// Copyright (c) 2022 Cogent Core. under the BSD-style License
// Copyright (c) 2017 Maxim Kupriianov <max@kc.vc>, under the MIT License

package asch

import (
	"image"
	"unsafe"

	vk "github.com/tomas-mraz/vulkan"
)

func ImgCompToUint8(val float32) uint8 {
	if val > 1.0 {
		val = 1.0
	}
	return uint8(val * float32(0xff))
}

// ImageFormat describes the size and vulkan format of an Image
// If Layers > 1, all must be the same size.
type ImageFormat struct {

	// Size of image
	Size image.Point

	// Image format -- FormatR8g8b8a8Srgb is a standard default
	Format vk.Format

	// number of samples -- set higher for Framebuffer rendering but otherwise default of SampleCount1Bit
	Samples vk.SampleCountFlagBits

	// number of layers for texture arrays
	Layers int
}

// NewImageFormat returns a new ImageFormat with default format and given size
// and number of layers
func NewImageFormat(width, height, layers int) *ImageFormat {
	im := &ImageFormat{}
	im.Defaults()
	im.Size = image.Point{width, height}
	im.Layers = layers
	return im
}

func (im *ImageFormat) Defaults() {
	im.Format = vk.FormatR8g8b8a8Srgb
	im.Samples = vk.SampleCount1Bit
	im.Layers = 1
}

// Set sets width, height and format
func (im *ImageFormat) Set(w, h int, ft vk.Format) {
	im.SetSize(w, h)
	im.Format = ft
}

// SetMultisample sets the number of multisampling to decrease aliasing
// 4 is typically sufficient.  Values must be power of 2.
func (im *ImageFormat) SetMultisample(nsamp int) {
	ns := vk.SampleCount1Bit
	switch nsamp {
	case 2:
		ns = vk.SampleCount2Bit
	case 4:
		ns = vk.SampleCount4Bit
	case 8:
		ns = vk.SampleCount8Bit
	case 16:
		ns = vk.SampleCount16Bit
	case 32:
		ns = vk.SampleCount32Bit
	case 64:
		ns = vk.SampleCount64Bit
	}
	im.Samples = ns
}

/////////////////////////////////////////////////////////////////////
// Image

// Image represents a vulkan image with an associated ImageView.
// The vulkan Image is in device memory, in an optimized format.
// There can also be an optional host-visible, plain pixel buffer
// which can be a pointer into a larger buffer or owned by the Image.
type Image struct {

	// name of the image -- e.g., same as Value name if used that way -- helpful for debugging -- set to filename if loaded from a file and otherwise empty
	Name string

	// bit flags for image state, for indicating nature of ownership and state
	Flags ImageFlags

	// format & size of image
	Format ImageFormat

	// vulkan image handle, in device memory
	Image vk.Image `display:"-"`

	// vulkan image view
	View vk.ImageView `display:"-"`

	// memory for image when we allocate it
	Mem vk.DeviceMemory `display:"-"`

	// keep track of device for destroying view
	Dev vk.Device `display:"-"`

	// host memory buffer representation of the image
	Host HostImage

	// pointer to our GPU
	GPU *GPU
}

// ConfigGoImage configures the image for storing an image
// of the given size, for images allocated in a shared host buffer.
// (i.e., not Var.TextureOwns).  Image format will be set to default
// unless format is already set.  Layers is number of separate images
// of given size allocated in a texture array.
// Once memory is allocated then SetGoImage can be called in a
// second pass.
func (im *Image) ConfigGoImage(sz image.Point, layers int) {
	if im.Format.Format != vk.FormatR8g8b8a8Srgb {
		im.Format.Defaults()
	}
	im.Format.Size = sz
	if layers <= 0 {
		layers = 1
	}
	im.Format.Layers = layers
}

// HostImage is the host representation of an Image
type HostImage struct {

	// size in bytes allocated for host representation of image
	Size int

	// buffer for host CPU-visible memory, for staging -- can be owned by us or managed by Memory (for Value)
	Buff vk.Buffer `display:"-"`

	// offset into host buffer, when Buff is Memory managed
	Offset int

	// host CPU-visible memory, for staging, when we manage our own memory
	Mem vk.DeviceMemory `display:"-"`

	// memory mapped pointer into host memory -- remains mapped
	Ptr unsafe.Pointer `display:"-"`
}

/////////////////////////////////////////////////////////////////////
// ImageFlags

// ImageFlags are bitflags for Image state
type ImageFlags int64 //enums:bitflag -trim-prefix Image
