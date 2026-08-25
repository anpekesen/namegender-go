# GenderScope Go

```sh
go get github.com/genderscope/genderscope-go
```

```go
client := genderscope.New(os.Getenv("GENDERSCOPE_API_KEY"))
result, err := client.Name(ctx, "Ayşe", genderscope.Options{Country: "TR"})
```
