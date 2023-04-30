package server

func removeElement[T any](slice []T, index int) []T {
	return append(slice[:index], slice[index+1:]...)
}

func removeElementByIndex[T any](slice []T, index int, indicesMap map[int]int) []T {
	sliceLen := len(slice)
	sliceLastIndex := sliceLen - 1

	if index != sliceLastIndex {
		slice[index] = slice[sliceLastIndex]
		indicesMap[sliceLastIndex] = indicesMap[index]
	}

	return slice[:sliceLastIndex]
}

func removeManyElementsByIndices[T any](slice []T, indices []int) []T {
	indicesMap := make(map[int]int)

	for _, index := range indices {
		indicesMap[index] = index
	}

	outputSlice := slice

	for _, index := range indices {
		mappedIndex := indicesMap[index]

		if mappedIndex < 0 || mappedIndex >= len(outputSlice) {
			continue
		}

		outputSlice = removeElementByIndex(outputSlice, indicesMap[index], indicesMap)
	}

	return outputSlice
}
