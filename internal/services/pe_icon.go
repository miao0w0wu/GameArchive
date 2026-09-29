package services

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"debug/pe"
)

type peResources struct {
	data       []byte
	rootOffset uint32
	rootSize   uint32
	sections   []*pe.Section
}

func extractPEIcon(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取游戏可执行文件信息失败: %w", err)
	}
	if info.Size() <= 0 || info.Size() > 512<<20 {
		return nil, errors.New("游戏可执行文件超过图标提取大小限制")
	}
	file, err := pe.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取游戏可执行文件失败: %w", err)
	}
	defer file.Close()

	var resourceDirectory pe.DataDirectory
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		resourceDirectory = header.DataDirectory[2]
	case *pe.OptionalHeader64:
		resourceDirectory = header.DataDirectory[2]
	default:
		return nil, errors.New("不支持的 PE 文件格式")
	}
	if resourceDirectory.VirtualAddress == 0 || resourceDirectory.Size == 0 {
		return nil, errors.New("可执行文件不包含图标资源")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取游戏可执行文件资源失败: %w", err)
	}
	resources := peResources{
		data:     data,
		rootSize: resourceDirectory.Size,
		sections: file.Sections,
	}
	root, ok := resources.rvaOffset(resourceDirectory.VirtualAddress)
	if !ok {
		return nil, errors.New("PE 资源目录无效")
	}
	resources.rootOffset = root

	groupDirectory, ok := resources.resourceID(0, 14)
	if !ok || groupDirectory&0x80000000 == 0 {
		return nil, errors.New("可执行文件不包含图标组")
	}
	groupEntry, ok := resources.firstResourceChild(groupDirectory & 0x7fffffff)
	if !ok || groupEntry&0x80000000 == 0 {
		return nil, errors.New("可执行文件图标组无效")
	}
	groupDataEntry, ok := resources.firstResourceChild(groupEntry & 0x7fffffff)
	if !ok || groupDataEntry&0x80000000 != 0 {
		return nil, errors.New("可执行文件图标组数据无效")
	}
	groupData, ok := resources.resourceData(groupDataEntry)
	if !ok || len(groupData) < 20 || binary.LittleEndian.Uint16(groupData[2:4]) != 1 {
		return nil, errors.New("可执行文件图标组格式无效")
	}

	count := int(binary.LittleEndian.Uint16(groupData[4:6]))
	if count == 0 || count > 256 || len(groupData) < 6+count*14 {
		return nil, errors.New("可执行文件图标组内容无效")
	}
	iconDirectory, ok := resources.resourceID(0, 3)
	if !ok || iconDirectory&0x80000000 == 0 {
		return nil, errors.New("可执行文件不包含图标图像")
	}

	iconFile := make([]byte, 6+count*16)
	binary.LittleEndian.PutUint16(iconFile[2:4], 1)
	binary.LittleEndian.PutUint16(iconFile[4:6], uint16(count))
	payloads := make([][]byte, 0, count)
	offset := uint32(len(iconFile))
	for index := 0; index < count; index++ {
		groupIcon := groupData[6+index*14 : 6+(index+1)*14]
		iconID := uint32(binary.LittleEndian.Uint16(groupIcon[12:14]))
		iconEntry, ok := resources.resourceID(iconDirectory&0x7fffffff, iconID)
		if !ok || iconEntry&0x80000000 == 0 {
			continue
		}
		languageEntry, ok := resources.firstResourceChild(iconEntry & 0x7fffffff)
		if !ok || languageEntry&0x80000000 != 0 {
			continue
		}
		iconData, ok := resources.resourceData(languageEntry)
		if !ok || len(iconData) == 0 || len(iconData) > maxImageSize {
			continue
		}
		entry := iconFile[6+index*16 : 6+(index+1)*16]
		copy(entry[:12], groupIcon[:12])
		binary.LittleEndian.PutUint32(entry[12:16], offset)
		offset += uint32(len(iconData))
		payloads = append(payloads, iconData)
	}
	if len(payloads) == 0 || offset > maxImageSize {
		return nil, errors.New("无法读取可执行文件中的图标")
	}
	result := make([]byte, 0, offset)
	result = append(result, iconFile...)
	for _, payload := range payloads {
		result = append(result, payload...)
	}
	if _, err := validateImage(result); err != nil {
		return nil, fmt.Errorf("可执行文件图标格式无效: %w", err)
	}
	return result, nil
}

func (r *peResources) resourceID(directoryOffset, id uint32) (uint32, bool) {
	offset, ok := r.rootRelativeOffset(directoryOffset, 16)
	if !ok {
		return 0, false
	}
	named := binary.LittleEndian.Uint16(r.data[offset+12 : offset+14])
	ids := binary.LittleEndian.Uint16(r.data[offset+14 : offset+16])
	entryOffset := offset + 16 + int(named)*8
	if entryOffset+int(ids)*8 > len(r.data) {
		return 0, false
	}
	for index := 0; index < int(ids); index++ {
		entry := r.data[entryOffset+index*8 : entryOffset+(index+1)*8]
		name := binary.LittleEndian.Uint32(entry[:4])
		if name&0x80000000 == 0 && name&0xffff == id {
			return binary.LittleEndian.Uint32(entry[4:8]), true
		}
	}
	return 0, false
}

func (r *peResources) firstResourceChild(directoryOffset uint32) (uint32, bool) {
	offset, ok := r.rootRelativeOffset(directoryOffset, 16)
	if !ok {
		return 0, false
	}
	named := binary.LittleEndian.Uint16(r.data[offset+12 : offset+14])
	ids := binary.LittleEndian.Uint16(r.data[offset+14 : offset+16])
	count := int(named) + int(ids)
	entryOffset := offset + 16
	if entryOffset+count*8 > len(r.data) || count == 0 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(r.data[entryOffset+4 : entryOffset+8]), true
}

func (r *peResources) resourceData(entry uint32) ([]byte, bool) {
	offset, ok := r.rootRelativeOffset(entry, 16)
	if !ok {
		return nil, false
	}
	rva := binary.LittleEndian.Uint32(r.data[offset : offset+4])
	size := binary.LittleEndian.Uint32(r.data[offset+4 : offset+8])
	if size == 0 || size > maxImageSize {
		return nil, false
	}
	dataOffset, ok := r.rvaOffset(rva)
	if !ok || uint64(dataOffset)+uint64(size) > uint64(len(r.data)) {
		return nil, false
	}
	return r.data[dataOffset : dataOffset+size], true
}

func (r *peResources) rootRelativeOffset(offset, size uint32) (int, bool) {
	if offset > r.rootSize || size > r.rootSize-offset {
		return 0, false
	}
	absolute := uint64(r.rootOffset) + uint64(offset)
	if absolute+uint64(size) > uint64(len(r.data)) {
		return 0, false
	}
	return int(absolute), true
}

func (r *peResources) rvaOffset(rva uint32) (uint32, bool) {
	for _, section := range r.sections {
		if rva < section.VirtualAddress {
			continue
		}
		delta := rva - section.VirtualAddress
		if delta >= section.Size {
			continue
		}
		offset := uint64(section.Offset) + uint64(delta)
		if offset >= uint64(len(r.data)) {
			return 0, false
		}
		return uint32(offset), true
	}
	return 0, false
}
