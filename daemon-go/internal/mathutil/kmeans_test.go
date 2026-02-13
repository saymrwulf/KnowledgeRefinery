package mathutil

import (
	"testing"
)

func TestKMeansEmpty(t *testing.T) {
	labels, centroids := KMeans(nil, 3, 100)
	if labels != nil || centroids != nil {
		t.Error("expected nil for empty input")
	}
}

func TestKMeansFewerThanK(t *testing.T) {
	vecs := [][]float32{
		{1, 0}, {0, 1},
	}
	labels, centroids := KMeans(vecs, 5, 100)
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	if len(centroids) != 2 {
		t.Fatalf("expected 2 centroids, got %d", len(centroids))
	}
	// Each point should be its own cluster
	if labels[0] != 0 || labels[1] != 1 {
		t.Errorf("expected labels [0,1], got %v", labels)
	}
}

func TestKMeansTwoClusters(t *testing.T) {
	// Two well-separated clusters
	vecs := [][]float32{
		{0, 0}, {0.1, 0.1}, {-0.1, 0.1},
		{10, 10}, {10.1, 10.1}, {9.9, 10.1},
	}
	labels, centroids := KMeans(vecs, 2, 100)
	if len(labels) != 6 {
		t.Fatalf("expected 6 labels, got %d", len(labels))
	}
	if len(centroids) != 2 {
		t.Fatalf("expected 2 centroids, got %d", len(centroids))
	}
	// First 3 should be in the same cluster, last 3 in another
	if labels[0] != labels[1] || labels[1] != labels[2] {
		t.Errorf("first cluster not consistent: %v", labels)
	}
	if labels[3] != labels[4] || labels[4] != labels[5] {
		t.Errorf("second cluster not consistent: %v", labels)
	}
	if labels[0] == labels[3] {
		t.Errorf("clusters should be different: %v", labels)
	}
}

func TestKMeansOneClusters(t *testing.T) {
	vecs := [][]float32{
		{1, 1}, {1.1, 1}, {1, 1.1},
	}
	labels, centroids := KMeans(vecs, 1, 100)
	if len(labels) != 3 || len(centroids) != 1 {
		t.Fatalf("expected 3 labels and 1 centroid")
	}
	for _, l := range labels {
		if l != 0 {
			t.Errorf("all labels should be 0 with k=1, got %d", l)
		}
	}
}
