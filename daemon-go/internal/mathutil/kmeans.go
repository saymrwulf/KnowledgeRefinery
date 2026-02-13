package mathutil

import (
	"math"
	"math/rand"
)

// KMeans performs k-means++ clustering. Returns (labels, centroids).
func KMeans(vectors [][]float32, k int, maxIter int) ([]int, [][]float32) {
	n := len(vectors)
	if n == 0 {
		return nil, nil
	}
	dim := len(vectors[0])

	if n <= k {
		labels := make([]int, n)
		centroids := make([][]float32, n)
		for i := range n {
			labels[i] = i
			centroids[i] = make([]float32, dim)
			copy(centroids[i], vectors[i])
		}
		return labels, centroids
	}

	// K-means++ initialization
	centroids := make([][]float32, k)
	idx := rand.Intn(n)
	centroids[0] = make([]float32, dim)
	copy(centroids[0], vectors[idx])

	for i := 1; i < k; i++ {
		dists := make([]float64, n)
		for j := 0; j < n; j++ {
			minDist := math.MaxFloat64
			for ci := 0; ci < i; ci++ {
				d := sqDist(vectors[j], centroids[ci])
				if d < minDist {
					minDist = d
				}
			}
			dists[j] = minDist
		}

		// Weighted random selection
		total := 0.0
		for _, d := range dists {
			total += d
		}
		if total == 0 {
			centroids[i] = make([]float32, dim)
			copy(centroids[i], vectors[rand.Intn(n)])
			continue
		}
		threshold := rand.Float64() * total
		cumsum := 0.0
		chosen := 0
		for j, d := range dists {
			cumsum += d
			if cumsum >= threshold {
				chosen = j
				break
			}
		}
		centroids[i] = make([]float32, dim)
		copy(centroids[i], vectors[chosen])
	}

	// Iterate
	labels := make([]int, n)
	for iter := 0; iter < maxIter; iter++ {
		// Assign
		changed := false
		for i := 0; i < n; i++ {
			bestK := 0
			bestDist := sqDist(vectors[i], centroids[0])
			for ci := 1; ci < k; ci++ {
				d := sqDist(vectors[i], centroids[ci])
				if d < bestDist {
					bestDist = d
					bestK = ci
				}
			}
			if labels[i] != bestK {
				labels[i] = bestK
				changed = true
			}
		}
		if !changed {
			break
		}

		// Update centroids
		counts := make([]int, k)
		newCentroids := make([][]float32, k)
		for ci := range k {
			newCentroids[ci] = make([]float32, dim)
		}
		for i := 0; i < n; i++ {
			ci := labels[i]
			counts[ci]++
			for d := 0; d < dim; d++ {
				newCentroids[ci][d] += vectors[i][d]
			}
		}
		for ci := range k {
			if counts[ci] > 0 {
				for d := 0; d < dim; d++ {
					newCentroids[ci][d] /= float32(counts[ci])
				}
				centroids[ci] = newCentroids[ci]
			}
		}
	}

	return labels, centroids
}

func sqDist(a, b []float32) float64 {
	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}
	return sum
}
