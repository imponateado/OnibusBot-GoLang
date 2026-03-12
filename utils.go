package main

import (
	"math"
)

const (
	PI              = math.Pi
	SM_A            = 6378137.0
	SM_B            = 6356752.314
	SM_ECC_SQUARED  = 6.69437999013e-03
	UTM_SCALE_FACTOR = 0.9996
)

func DegToRad(deg float64) float64 {
	return deg / 180.0 * PI
}

func RadToDeg(rad float64) float64 {
	return rad / PI * 180.0
}

func ArcLengthOfMeridian(phi float64) float64 {
	n := (SM_A - SM_B) / (SM_A + SM_B)
	alpha := ((SM_A + SM_B) / 2.0) * (1.0 + (math.Pow(n, 2.0) / 4.0) + (math.Pow(n, 4.0) / 64.0))
	beta := (-3.0 * n / 2.0) + (9.0 * math.Pow(n, 3.0) / 16.0) + (-3.0 * math.Pow(n, 5.0) / 32.0)
	gamma := (15.0 * math.Pow(n, 2.0) / 16.0) + (-15.0 * math.Pow(n, 4.0) / 32.0)
	delta := (-35.0 * math.Pow(n, 3.0) / 48.0) + (105.0 * math.Pow(n, 5.0) / 256.0)
	epsilon := (315.0 * math.Pow(n, 4.0) / 512.0)
	result := alpha * (phi + (beta * math.Sin(2.0*phi)) + (gamma * math.Sin(4.0*phi)) +
		(delta * math.Sin(6.0*phi)) + (epsilon * math.Sin(8.0*phi)))

	return result
}

func UTMCentralMeridian(zone int) float64 {
	return DegToRad(-183.0 + (float64(zone) * 6.0))
}

func FootpointLatitude(y float64) float64 {
	n := (SM_A - SM_B) / (SM_A + SM_B)

	alpha_ := ((SM_A + SM_B) / 2.0) *
		(1 + (math.Pow(n, 2.0) / 4) + (math.Pow(n, 4.0) / 64))

	y_ := y / alpha_

	beta_ := (3.0 * n / 2.0) + (-27.0 * math.Pow(n, 3.0) / 32.0) +
		(269.0 * math.Pow(n, 5.0) / 512.0)

	gamma_ := (21.0 * math.Pow(n, 2.0) / 16.0) +
		(-55.0 * math.Pow(n, 4.0) / 32.0)

	delta_ := (151.0 * math.Pow(n, 3.0) / 96.0) +
		(-417.0 * math.Pow(n, 5.0) / 128.0)

	epsilon_ := (1097.0 * math.Pow(n, 4.0) / 512.0)

	result := y_ + (beta_ * math.Sin(2.0*y_)) +
		(gamma_ * math.Sin(4.0*y_)) +
		(delta_ * math.Sin(6.0*y_)) +
		(epsilon_ * math.Sin(8.0*y_))

	return result
}

func MapXYToLatLon(x, y, lambda0 float64) (phi, lambda float64) {
	phif := FootpointLatitude(y)

	ep2 := (math.Pow(SM_A, 2.0) - math.Pow(SM_B, 2.0)) / math.Pow(SM_B, 2.0)
	cf := math.Cos(phif)
	nuf2 := ep2 * math.Pow(cf, 2.0)
	Nf := math.Pow(SM_A, 2.0) / (SM_B * math.Sqrt(1+nuf2))
	Nfpow := Nf

	tf := math.Tan(phif)
	tf2 := tf * tf
	tf4 := tf2 * tf2

	x1frac := 1.0 / (Nfpow * cf)

	Nfpow *= Nf
	x2frac := tf / (2.0 * Nfpow)

	Nfpow *= Nf
	x3frac := 1.0 / (6.0 * Nfpow * cf)

	Nfpow *= Nf
	x4frac := tf / (24.0 * Nfpow)

	Nfpow *= Nf
	x5frac := 1.0 / (120.0 * Nfpow * cf)

	Nfpow *= Nf
	x6frac := tf / (720.0 * Nfpow)

	Nfpow *= Nf
	x7frac := 1.0 / (5040.0 * Nfpow * cf)

	Nfpow *= Nf
	x8frac := tf / (40320.0 * Nfpow)

	x2poly := -1.0 - nuf2
	x3poly := -1.0 - 2*tf2 - nuf2
	x4poly := 5.0 + 3.0*tf2 + 6.0*nuf2 - 6.0*tf2*nuf2 -
		3.0*(nuf2*nuf2) - 9.0*tf2*(nuf2*nuf2)
	x5poly := 5.0 + 28.0*tf2 + 24.0*tf4 + 6.0*nuf2 + 8.0*tf2*nuf2
	x6poly := -61.0 - 90.0*tf2 - 45.0*tf4 - 107.0*nuf2 + 162.0*tf2*nuf2
	x7poly := -61.0 - 662.0*tf2 - 1320.0*tf4 - 720.0*(tf4*tf2)
	x8poly := 1385.0 + 3633.0*tf2 + 4095.0*tf4 + 1575*(tf4*tf2)

	phi = phif + x2frac*x2poly*(x*x) +
		x4frac*x4poly*math.Pow(x, 4.0) +
		x6frac*x6poly*math.Pow(x, 6.0) +
		x8frac*x8poly*math.Pow(x, 8.0)

	lambda = lambda0 + x1frac*x +
		x3frac*x3poly*math.Pow(x, 3.0) +
		x5frac*x5poly*math.Pow(x, 5.0) +
		x7frac*x7poly*math.Pow(x, 7.0)

	return
}

func UTMToLatLon(x, y float64, zone int, southHemi bool) (float64, float64) {
	x -= 500000.0
	x /= UTM_SCALE_FACTOR

	if southHemi {
		y -= 10000000.0
	}

	y /= UTM_SCALE_FACTOR

	cmeridian := UTMCentralMeridian(zone)
	phi, lambda := MapXYToLatLon(x, y, cmeridian)

	return RadToDeg(phi), RadToDeg(lambda)
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
