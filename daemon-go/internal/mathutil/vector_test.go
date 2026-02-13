package mathutil

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	sim := CosineSimilarity(a, a)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("expected 1.0, got %f", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 1e-6 {
		t.Errorf("expected -1.0, got %f", sim)
	}
}

func TestCosineSimilarityZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("expected 0.0 for zero vector, got %f", sim)
	}
}

func TestNormalizeUnit(t *testing.T) {
	v := []float32{3, 4, 0}
	n := Normalize(v)
	// Should be [0.6, 0.8, 0]
	if math.Abs(float64(n[0])-0.6) > 1e-5 || math.Abs(float64(n[1])-0.8) > 1e-5 {
		t.Errorf("unexpected normalized vector: %v", n)
	}
	// Magnitude should be 1.0
	mag := DotProduct(n, n)
	if math.Abs(mag-1.0) > 1e-5 {
		t.Errorf("normalized vector magnitude %f != 1.0", mag)
	}
}

func TestNormalizeZero(t *testing.T) {
	v := []float32{0, 0, 0}
	n := Normalize(v)
	for i, x := range n {
		if x != 0 {
			t.Errorf("expected zero at index %d, got %f", i, x)
		}
	}
}

func TestDotProduct(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}
	// 1*4 + 2*5 + 3*6 = 4 + 10 + 18 = 32
	got := DotProduct(a, b)
	if math.Abs(got-32.0) > 1e-6 {
		t.Errorf("expected 32.0, got %f", got)
	}
}
