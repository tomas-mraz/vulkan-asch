//go:build android

package asch

import (
	vk "github.com/tomas-mraz/vulkan"
)

func NewAndroidSurface(instance vk.Instance, windowPtr uintptr) vk.Surface {
	surface := vk.Surface{}
	result := vk.CreateWindowSurface(instance, windowPtr, nil, &surface)
	if err := vk.Error(result); err != nil {
		panic(err)
	}
	return surface
}
