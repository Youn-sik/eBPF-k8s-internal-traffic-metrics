package main

import (
	"testing"
)

func TestL4Filter_NewL4Filter(t *testing.T) {
	tests := []struct {
		name           string
		customExcludes string
		wantCount      int
		wantContains   []string
	}{
		{
			name:           "default only",
			customExcludes: "",
			wantCount:      1, // kubelet
			wantContains:   []string{"kubelet"},
		},
		{
			name:           "with custom excludes",
			customExcludes: "coredns,nginx",
			wantCount:      3, // kubelet + coredns + nginx
			wantContains:   []string{"kubelet", "coredns", "nginx"},
		},
		{
			name:           "with spaces",
			customExcludes: " coredns , nginx ",
			wantCount:      3,
			wantContains:   []string{"kubelet", "coredns", "nginx"},
		},
		{
			name:           "with empty parts",
			customExcludes: "coredns,,nginx,",
			wantCount:      3,
			wantContains:   []string{"kubelet", "coredns", "nginx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewL4Filter(tt.customExcludes)

			if len(filter.excludeComms) != tt.wantCount {
				t.Errorf("got %d excludes, want %d", len(filter.excludeComms), tt.wantCount)
			}

			for _, want := range tt.wantContains {
				if _, ok := filter.excludeComms[want]; !ok {
					t.Errorf("filter should contain %q", want)
				}
			}
		})
	}
}

func TestL4Filter_ShouldExclude(t *testing.T) {
	filter := NewL4Filter("coredns,nginx")

	tests := []struct {
		comm        string
		wantExclude bool
		wantMatch   string
	}{
		// Exact matches
		{"kubelet", true, "kubelet"},
		{"coredns", true, "coredns"},
		{"nginx", true, "nginx"},

		// Prefix matches
		{"kubelet-runner", true, "kubelet"},
		{"coredns-12345", true, "coredns"},
		{"nginx-proxy", true, "nginx"},

		// Case insensitive
		{"KUBELET", true, "kubelet"},
		{"CoreDNS", true, "coredns"},
		{"NGINX", true, "nginx"},

		// Should not exclude
		{"python", false, ""},
		{"java", false, ""},
		{"go", false, ""},
		{"myapp", false, ""},

		// Partial match in wrong position (should not match)
		{"my-kubelet-app", false, ""}, // kubelet is not prefix
	}

	for _, tt := range tests {
		t.Run(tt.comm, func(t *testing.T) {
			excluded, match := filter.ShouldExclude(tt.comm)

			if excluded != tt.wantExclude {
				t.Errorf("ShouldExclude(%q) = %v, want %v", tt.comm, excluded, tt.wantExclude)
			}

			if excluded && match != tt.wantMatch {
				t.Errorf("ShouldExclude(%q) matched %q, want %q", tt.comm, match, tt.wantMatch)
			}
		})
	}
}

func TestL4Filter_ExcludeCommsList(t *testing.T) {
	filter := NewL4Filter("coredns,nginx")
	list := filter.ExcludeCommsList()

	if len(list) != 3 {
		t.Errorf("got %d items, want 3", len(list))
	}

	// Check all expected items are present
	expected := map[string]bool{"kubelet": false, "coredns": false, "nginx": false}
	for _, item := range list {
		if _, ok := expected[item]; ok {
			expected[item] = true
		}
	}

	for item, found := range expected {
		if !found {
			t.Errorf("expected %q in list", item)
		}
	}
}
