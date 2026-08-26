# NameGender Go

```sh
go get github.com/anpekesen/namegender-go
```

```go
client := namegender.New(os.Getenv("NAMEGENDER_API_KEY"))
result, err := client.Name(ctx, "Ayşe", namegender.Options{Country: "TR"})
```
