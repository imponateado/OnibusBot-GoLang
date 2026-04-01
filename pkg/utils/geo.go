package utils

import (
	"math"
)

const (
	PI = math.Pi
)

func DegToRad(deg float64) float64 {
	return deg / 180.0 * PI
}

func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := DegToRad(lat2 - lat1)
	dLon := DegToRad(lon2 - lon1)

	rLat1 := DegToRad(lat1)
	rLat2 := DegToRad(lat2)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(rLat1)*math.Cos(rLat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return 6371 * c // Radius of Earth in km
}
