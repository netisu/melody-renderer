package aeno

import (
	"fmt"
)

const (
	ver = "a.1"
)

func aeno() {
	fmt.Println("Aeno", ver, "Aeo Zatoichi Bax")
	subject := []VectorW{
		{0, 0, 0, 1}, {2, 0, 0, 1}, {2, 2, 0, 1}, {0, 2, 0, 1},
	}
	clip := []VectorW{
		{1, 1, 0, 1}, {2, 1, 0, 1}, {2, 2, 0, 1}, {1, 2, 0, 1},
	}

	result := WeilerAtherton(subject, clip)

	fmt.Println("Resulting polygons:")
	for _, polygon := range result {
		fmt.Println(polygon)
	}
	
	return;
}
