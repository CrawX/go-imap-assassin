// SPDX-License-Identifier: GPL-3.0-or-later
package imapconnection

func u32(val int) uint32 {
	return uint32(val)
}

func u32a(val ...int) []uint32 {
	a := []uint32{}
	for _, v := range val {
		a = append(a, u32(v))
	}

	return a
}
