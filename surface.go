// Copyright (c) 2025 Cubyte.online under the AGPL License
// Copyright (c) 2022 Cogent Core. under the BSD-style License
// Copyright (c) 2017 Maxim Kupriianov <max@kc.vc>, under the MIT License

package asch

import (
	"fmt"

	vk "github.com/tomas-mraz/vulkan"
)

var Debug = false

// Surface manages the physical device for the visible image
// of a window surface, and the swapchain for presenting images.
type Surface struct {

	// pointer to gpu device, for convenience
	//GPU *vk.GPU

	// device for this surface -- each window surface has its own device, configured for that surface
	Device vk.Device

	// vulkan handle for surface
	Surface vk.Surface

	// vulkan handle for swapchain
	Swapchain vk.Swapchain

	// the Render for this Surface, typically from a System
	Render *Render

	// has the current swapchain image format and dimensions
	Format ImageFormat

	// ordered list of surface formats to select
	DesiredFormats []vk.Format

	// the number of frames to maintain in the swapchain -- e.g., 2 = double-buffering, 3 = triple-buffering -- initially set to a requested amount, and after Init reflects the actual number
	NFrames int

	// Framebuffers representing the visible Image owned by the Surface -- we iterate through these in rendering subsequent frames
	Frames []*vk.Framebuffer

	// semaphore used internally for waiting on acquisition of the next frame
	ImageAcquired vk.Semaphore

	// semaphore that surface user can wait on, will be activated when the image has been acquired in AcquireNextFrame method
	RenderDone vk.Semaphore

	// fence for rendering command running
	RenderFence vk.Fence

	// NeedsConfig is whether the surface needs to be configured again without freeing the swapchain.
	// This is set internally to allow for correct recovery from sudden minimization events that are
	// only detected at the point of swapchain reconfiguration.
	NeedsConfig bool
}

// NewSurface returns a new surface initialized for given GPU and vulkan
// Surface handle, obtained from a valid window.
func NewSurface(device *vk.Device, vsurf vk.Surface, swapchain vk.Swapchain) *Surface {
	sf := &Surface{}
	sf.Defaults()
	sf.Init(device, vsurf, swapchain)
	return sf
}

func (sf *Surface) Defaults() {
	sf.NFrames = 3 // requested, will be updated with actual
	sf.Format.Defaults()
	sf.Format.Set(1024, 768, vk.FormatR8g8b8a8Srgb)
	sf.Format.SetMultisample(4) // good default
	sf.DesiredFormats = []vk.Format{
		vk.FormatR8g8b8a8Srgb,
		vk.FormatB8g8r8a8Srgb,
	}
}

// Init initializes the device and all other resources for the surface
// based on the vulkan surface handle which must be obtained from the
// OS-specific window, created first (e.g., via glfw)
func (sf *Surface) Init(device *vk.Device, vsurf vk.Surface, swapchain vk.Swapchain) error {
	sf.Device = device
	sf.Surface = vsurf
	sf.Swapchain = swapchain
	sf.Render = nil
	return nil
}

// ConfigSwapchain configures the swapchain for surface.
// This assumes that all existing items have been destroyed.
// It returns false if the swapchain size is zero.
func (sf *Surface) ConfigSwapchain() bool {
	return false
}

// FreeSwapchain frees any existing swawpchain (for ReInit or Destroy)
func (sf *Surface) FreeSwapchain() {
}

// ReConfigSwapchain does a re-initialize of swapchain, freeing existing.
// This must be called when the window is resized.
// It returns false if the swapchain size is zero.
func (sf *Surface) ReConfigSwapchain() bool {
	return false
}

// SetRender sets the Render and updates frames accordingly
func (sf *Surface) SetRender(rp *Render) {
	sf.Render = rp
	for _, fr := range sf.Frames {
		fr.ConfigRender(rp)
	}
}

// ReConfigFrames re-configures the Famebuffers
// using exiting settings.  Assumes ConfigSwapchain has been called.
func (sf *Surface) ReConfigFrames() {
}

func (sf *Surface) Destroy() {
}

// AcquireNextImage gets the next frame index to render to.
// It automatically handles any issues with out-of-date swapchain.
// It triggers the ImageAcquired semaphore when image actually acquired.
// Must call SubmitRender with command to launch command contingent
// on that semaphore. It returns false if the swapchain size is zero.
func (sf *Surface) AcquireNextImage() (uint32, bool) {
	dev := sf.Device.Device
	for {
		vk.WaitForFences(dev, 1, []vk.Fence{sf.RenderFence}, vk.True, vk.MaxUint64)
		vk.ResetFences(dev, 1, []vk.Fence{sf.RenderFence})
		var idx uint32
		if sf.NeedsConfig {
			// we must skip FreeSwapchain for NeedsConfig
			if !sf.ConfigSwapchain() {
				if Debug {
					fmt.Println("vgpu.Surface.AcquireNextImage: bailing on ConfigSwapchain caused by NeedsConfig (somewhat unexpected)")
				}
				return idx, false
			}
			sf.Render.SetSize(sf.Format.Size)
			sf.ReConfigFrames()
			sf.NeedsConfig = false
			if Debug {
				fmt.Println("vgpu.Surface.AcquireNextImage: did NeedsConfig update")
			}
			continue
		}
		ret := vk.AcquireNextImage(dev, sf.Swapchain, vk.MaxUint64, sf.ImageAcquired, vk.NullFence, &idx)
		switch ret {
		case vk.ErrorOutOfDate, vk.Suboptimal:
			if !sf.ReConfigSwapchain() {
				if Debug {
					fmt.Println("vgpu.Surface.AcquireNextImage: bailing on zero swapchain size")
				}
				sf.NeedsConfig = true
				return idx, false
			}
			if Debug {
				fmt.Printf("vgpu.Surface.AcquireNextImage: new format: %#v\n", sf.Format)
			}
			continue // try again
		case vk.Success:
			return idx, true
		default:
			IfPanic(NewError(ret))
			return idx, true
		}
	}
}

// SubmitRender submits a rendering command that must have been added
// to the given command buffer, calling CmdEnd on the buffer first.
// This buffer triggers the associated Fence logic to control the
// sequencing of render commands over time.
// The ImageAcquired semaphore before the command is run.
func (sf *Surface) SubmitRender(cmd vk.CommandBuffer) {
	CmdEnd(cmd)
	ret := vk.QueueSubmit(sf.Device.Queue, 1, []vk.SubmitInfo{{
		SType: vk.StructureTypeSubmitInfo,
		PWaitDstStageMask: []vk.PipelineStageFlags{
			vk.PipelineStageFlags(vk.PipelineStageColorAttachmentOutputBit),
		},
		WaitSemaphoreCount:   1,
		PWaitSemaphores:      []vk.Semaphore{sf.ImageAcquired},
		CommandBufferCount:   1,
		PCommandBuffers:      []vk.CommandBuffer{cmd},
		SignalSemaphoreCount: 1,
		PSignalSemaphores:    []vk.Semaphore{sf.RenderDone},
	}}, sf.RenderFence)
	IfPanic(NewError(ret))
}

// PresentImage waits on the RenderDone semaphore to present the
// rendered image to the surface, for the given frame index,
// as returned by AcquireNextImage.
func (sf *Surface) PresentImage(frameIndex uint32) error {
	ret := vk.QueuePresent(sf.Device.Queue, &vk.PresentInfo{
		SType:              vk.StructureTypePresentInfo,
		WaitSemaphoreCount: 1,
		PWaitSemaphores:    []vk.Semaphore{sf.RenderDone},
		SwapchainCount:     1,
		PSwapchains:        []vk.Swapchain{sf.Swapchain},
		PImageIndices:      []uint32{frameIndex},
	})

	switch ret {
	case vk.ErrorOutOfDate, vk.Suboptimal:
		if Debug {
			fmt.Println("vgpu.Surface.PresentImage: did not render due to out of date or suboptimal swapchain")
		}
		return nil
	case vk.Success:
		return nil
	default:
		return NewError(ret)
	}
}
