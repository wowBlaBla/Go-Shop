package handler

import "testing"

func TestParseArguments(t *testing.T) {
	for _, example := range []struct{
		In string
		Out []string
	}{
		{In: ``, Out: []string{}},
		{In: `"key1" "value1" "key2" 2`, Out: []string{"key1", "value1", "key2", "2"}},
		{In: `"key1"    "value1" "key2" 2`, Out: []string{"key1", "value1", "key2", "2"}},
		{In: `"key1"    "value1   " "key2" 2`, Out: []string{"key1", "value1   ", "key2", "2"}},
		{In: `"key1" "value1" "key2" 2 "key3" 33`, Out: []string{"key1", "value1", "key2", "2", "key3", "33"}},
		{In: `"key1" "value1" "key2 2  2" 2 "key3 33  33" 33`, Out: []string{"key1", "value1", "key2 2  2", "2", "key3 33  33", "33"}},
		{In: `"key1" "val\"ue\""`, Out: []string{"key1", `val\"ue\"`}},
		{In: `"key1" "val\"ue\"1"`, Out: []string{"key1", `val\"ue\"1`}},
	}{
		out := ParseArguments(example.In)
		if len(out) == len(example.Out) {
			for i := 0; i < len(out); i++ {
				if out[i] != example.Out[i] {
					t.Errorf("%v != %v", out[i], example.Out[i])
				}
			}
		}else{
			t.Errorf("Output mismatch")
		}
	}
}