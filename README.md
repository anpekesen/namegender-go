# NameGender Go

```sh
go get github.com/anpekesen/namegender-go
```

```go
client := namegender.New(os.Getenv("NAMEGENDER_API_KEY"))
result, err := client.Name(ctx, "Ayşe", namegender.Options{Country: "TR"})
```

## Country distribution

Which countries a name is recorded in. This is not a country-of-origin or
ethnicity inference: `Registrations` is counted volume, comparable only among
the countries that publish counted birth statistics, and `AttestedIn` is
presence with no weight attached. Show `Basis.Note` next to any percentage.

```go
dist, err := client.Countries(ctx, "Mehmet", namegender.CountriesOptions{Limit: 10})
for _, r := range dist.Registrations {
	fmt.Printf("%s %.2f%%\n", r.Country, r.Share)
}
fmt.Println(strings.Join(dist.AttestedIn, ", "))
```
