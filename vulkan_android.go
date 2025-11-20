//go:build android

package asch

import (
	vk "github.com/tomas-mraz/vulkan"
)

func NewSurface(instance vk.Instance, window uintptr) vk.Surface {
	surface := vk.Surface{}
	result := vk.CreateWindowSurface(instance, window, nil, &surface)
	if err := vk.Error(result); err != nil {
		panic(err)
	}
	return surface
}
