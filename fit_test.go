package main

import "testing"

func TestParseFitOutput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    fitResult
		wantErr bool
	}{
		{
			name: "minimal",
			in:   "-c 16384 -ngl 28\n",
			want: fitResult{ctxSize: 16384, nGPULayers: 28},
		},
		{
			name: "with tensor split",
			in:   "-c 4096 -ngl 0 -ts 3,1",
			want: fitResult{ctxSize: 4096, nGPULayers: 0, tensorSplit: "3,1"},
		},
		{
			name: "quoted override-tensor",
			in:   `-c 4096 -ngl 12 -ot "LLM_FFN_EXPS=CPU"`,
			want: fitResult{ctxSize: 4096, nGPULayers: 12, overrideTensor: "LLM_FFN_EXPS=CPU"},
		},
		{
			name: "full",
			in:   `-c 8192 -ngl 20 -ts 1,2 -ot "A=CPU,B=CPU"`,
			want: fitResult{ctxSize: 8192, nGPULayers: 20, tensorSplit: "1,2", overrideTensor: "A=CPU,B=CPU"},
		},
		{
			name:    "empty",
			in:      "  \n",
			wantErr: true,
		},
		{
			name:    "unknown token",
			in:      "-c 4096 -wat 1",
			wantErr: true,
		},
		{
			name:    "missing value",
			in:      "-c",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFitOutput(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFitOutput(%q) err = nil, want error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFitOutput(%q) unexpected err: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseFitOutput(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSplitQuoted(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"-c 4096", []string{"-c", "4096"}},
		{`-ot "A=CPU,B=CPU"`, []string{"-ot", "A=CPU,B=CPU"}},
		{`-ot "A=CPU" -ngl 3`, []string{"-ot", "A=CPU", "-ngl", "3"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitQuoted(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitQuoted(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitQuoted(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseTuneArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantName  string
		wantKeys  map[string]string
		wantOrder []string
		wantErr   bool
	}{
		{
			name:     "name only",
			args:     []string{"Qwen2.5-X"},
			wantName: "Qwen2.5-X",
			wantKeys: map[string]string{},
		},
		{
			name:      "with forced keys",
			args:      []string{"Qwen2.5-X", "--n-cpu-moe", "28", "--ctx=32768"},
			wantName:  "Qwen2.5-X",
			wantKeys:  map[string]string{"n-cpu-moe": "28", "ctx-size": "32768"},
			wantOrder: []string{"n-cpu-moe", "ctx-size"},
		},
		{
			name:    "missing name",
			args:    []string{"--n-cpu-moe", "28"},
			wantErr: true,
		},
		{
			name:    "flag without value",
			args:    []string{"Qwen2.5-X", "--ctx"},
			wantErr: true,
		},
		{
			name:    "extra positional",
			args:    []string{"Qwen2.5-X", "extra"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, order, m, err := parseTuneArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTuneArgs(%v) err = nil, want error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTuneArgs(%v) unexpected err: %v", tt.args, err)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if len(m) != len(tt.wantKeys) {
				t.Errorf("keys = %v, want %v", m, tt.wantKeys)
			}
			for k, v := range tt.wantKeys {
				if m[k] != v {
					t.Errorf("key %q = %q, want %q", k, m[k], v)
				}
			}
			if tt.wantOrder != nil {
				for i, k := range tt.wantOrder {
					if i >= len(order) || order[i] != k {
						t.Errorf("order = %v, want %v", order, tt.wantOrder)
						break
					}
				}
			}
		})
	}
}
