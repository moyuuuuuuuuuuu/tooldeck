package main
import("encoding/json";"os")
func main(){var input map[string]any;if err:=json.NewDecoder(os.Stdin).Decode(&input);err!=nil{panic(err)};json.NewEncoder(os.Stdout).Encode(map[string]any{"echo":input["text"],"runtime":"go"})}
