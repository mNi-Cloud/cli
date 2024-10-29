package factory

import (
	"encoding/json"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mNi-Cloud/backend/common/pkg/mni"
	"github.com/mNi-Cloud/backend/common/pkg/mni/apigen/model"
	"github.com/mNi-Cloud/cli/internal/pkg/client"
	"github.com/urfave/cli/v2"
	"strconv"
)

type RestCommandFactory struct {
	rootUrl      string
	apiVersion   string
	resourceName string
	model        *model.Model
}

func (r RestCommandFactory) Command(name string) *cli.Command {
	return &cli.Command{
		Name: name,
		Subcommands: []*cli.Command{
			{
				Name:      "get",
				ArgsUsage: "<name>",
				Before:    TokenFunc(),
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() != 1 {
						cli.ShowSubcommandHelpAndExit(ctx, 1)
						return nil
					}

					rest := client.NewRestClient(ctx.String(r.rootUrl), r.apiVersion, r.resourceName)

					res, err := rest.Get(ctx.Context, ctx.Args().First(), &client.Params{Authorization: "Bearer " + ctx.String("token")})
					if err != nil {
						return err
					}

					if res.StatusCode != 200 {
						message := res.SingletonResponse.Body["message"]
						return cli.Exit(strconv.FormatInt(int64(res.StatusCode), 10)+": "+message.(string), 1)
					} else {
						r.displaySingle(ctx, res.SingletonResponse)
					}
					return nil
				},
			},
			{
				Name:   "list",
				Before: TokenFunc(),
				Action: func(ctx *cli.Context) error {
					rest := client.NewRestClient(ctx.String(r.rootUrl), r.apiVersion, r.resourceName)

					res, err := rest.List(ctx.Context, &client.Params{Authorization: "Bearer " + ctx.String("token")})
					if err != nil {
						return err
					}

					if res.StatusCode != 200 {
						message := res.SingletonResponse.Body["message"]
						return cli.Exit(strconv.FormatInt(int64(res.StatusCode), 10)+": "+message.(string), 1)
					} else {
						r.displayMultiple(ctx, res.MultitonResponse)
					}
					return nil
				},
			},
			{
				Name:   "create",
				Before: TokenFunc(),
				Flags:  r.appendFlags([]cli.Flag{}, "", r.model, false),
				Action: func(ctx *cli.Context) error {
					request := r.parseFlags(ctx, "", r.model, false)

					rest := client.NewRestClient(ctx.String(r.rootUrl), r.apiVersion, r.resourceName)

					res, err := rest.Create(ctx.Context, request, &client.Params{Authorization: "Bearer " + ctx.String("token")})
					if err != nil {
						return err
					}

					if res.StatusCode != 201 {
						message := res.SingletonResponse.Body["message"]
						return cli.Exit(strconv.FormatInt(int64(res.StatusCode), 10)+": "+message.(string), 1)
					} else {
						r.displaySingle(ctx, res.SingletonResponse)
					}
					return nil
				},
			},
			{
				Name:      "update",
				ArgsUsage: "<name>",
				Before:    TokenFunc(),
				Flags:     r.appendFlags([]cli.Flag{}, "", r.model, true),
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() != 1 {
						cli.ShowSubcommandHelpAndExit(ctx, 1)
						return nil
					}

					request := r.parseFlags(ctx, "", r.model, true)

					rest := client.NewRestClient(ctx.String(r.rootUrl), r.apiVersion, r.resourceName)

					res, err := rest.Patch(ctx.Context, ctx.Args().First(), request, &client.Params{Authorization: "Bearer " + ctx.String("token")})
					if err != nil {
						return err
					}

					if res.StatusCode != 200 {
						message := res.SingletonResponse.Body["message"]
						return cli.Exit(strconv.FormatInt(int64(res.StatusCode), 10)+": "+message.(string), 1)
					} else {
						r.displaySingle(ctx, res.SingletonResponse)
					}
					return nil
				},
			},
			{
				Name:      "delete",
				ArgsUsage: "<name>",
				Before:    TokenFunc(),
				Action: func(ctx *cli.Context) error {
					if ctx.NArg() != 1 {
						cli.ShowSubcommandHelpAndExit(ctx, 1)
						return nil
					}

					rest := client.NewRestClient(ctx.String(r.rootUrl), r.apiVersion, r.resourceName)

					res, err := rest.Delete(ctx.Context, ctx.Args().First(), &client.Params{Authorization: "Bearer " + ctx.String("token")})
					if err != nil {
						return err
					}

					if res.StatusCode != 204 {
						message := res.SingletonResponse.Body["message"]
						return cli.Exit(strconv.FormatInt(int64(res.StatusCode), 10)+": "+message.(string), 1)
					} else {
						fmt.Println("Deleted successfully")
					}
					return nil
				},
			},
		},
	}
}

func (r RestCommandFactory) parseFlags(ctx *cli.Context, prefix string, m *model.Model, update bool) map[string]interface{} {
	request := map[string]interface{}{}

	for _, fieldName := range m.FieldsOrder {
		field := m.Fields[fieldName]
		jsonFieldName := mni.ToLowerCamel(fieldName)

		if (!update && !field.ReadOnly) || (update && field.Mutable && !field.ReadOnly) {
			if field.Type == model.TypeString {
				if ctx.IsSet(prefix + jsonFieldName) {
					request[jsonFieldName] = ctx.String(prefix + jsonFieldName)
				}
			} else if field.Type == model.TypeSlice {
				if field.FieldSlice.Item.Type == model.TypeString {
					if ctx.IsSet(prefix + jsonFieldName) {
						request[jsonFieldName] = ctx.StringSlice(prefix + jsonFieldName)
					}
				} else if field.FieldSlice.Item.Type == model.TypeStruct {
					if ctx.IsSet(prefix + jsonFieldName) {
						request[jsonFieldName] = []map[string]interface{}{}
						for _, value := range ctx.StringSlice(prefix + jsonFieldName) {
							var obj map[string]interface{}
							err := json.Unmarshal([]byte(value), &obj)
							if err != nil {
								fmt.Println("Error parsing JSON of", prefix+jsonFieldName, ":", err)
							} else {
								request[jsonFieldName] = append(request[jsonFieldName].([]map[string]interface{}), obj)
							}
						}
					}
				} else {
					fmt.Println("Unsupported field type: slice of", field.FieldSlice.Item.Type)
				}
			} else if field.Type == model.TypeBool {
				if ctx.IsSet(prefix + jsonFieldName) {
					request[jsonFieldName] = ctx.Bool(prefix + jsonFieldName)
				}
			} else if field.Type == model.TypeStruct {
				request[jsonFieldName] = r.parseFlags(ctx, prefix+jsonFieldName+".", &field.FieldStruct.Model, update)
			} else if field.Type == model.TypeUint {
				if ctx.IsSet(prefix + jsonFieldName) {
					request[jsonFieldName] = ctx.Uint(prefix + jsonFieldName)
				}
			} else if field.Type == model.TypeInt {
				if ctx.IsSet(prefix + jsonFieldName) {
					request[jsonFieldName] = ctx.Int(prefix + jsonFieldName)
				}
			} else if field.Type == model.TypeFloat {
				if ctx.IsSet(prefix + jsonFieldName) {
					request[jsonFieldName] = ctx.Float64(prefix + jsonFieldName)
				}
			} else if field.Type == model.TypeTime {
				fmt.Println("Unsupported field type: Time")
			} else {
				fmt.Println("Unsupported field type:", field.Type)
			}
		}
	}

	return request
}

func (r RestCommandFactory) appendFlags(flags []cli.Flag, prefix string, m *model.Model, update bool) []cli.Flag {
	for _, fieldName := range m.FieldsOrder {
		field := m.Fields[fieldName]

		if (!update && !field.ReadOnly) || (update && field.Mutable && !field.ReadOnly) {
			if field.Type == model.TypeString {
				flags = append(flags, &cli.StringFlag{
					Name:     prefix + mni.ToLowerCamel(fieldName),
					Required: field.Required,
				})
			} else if field.Type == model.TypeSlice {
				if field.FieldSlice.Item.Type == model.TypeString {
					flags = append(flags, &cli.StringSliceFlag{
						Name:     prefix + mni.ToLowerCamel(fieldName),
						Required: field.Required,
					})
				} else if field.FieldSlice.Item.Type == model.TypeStruct {
					flags = append(flags, &cli.StringSliceFlag{
						Name:        prefix + mni.ToLowerCamel(fieldName),
						Required:    field.Required,
						DefaultText: "{}",
					})
				} else {
					fmt.Println("Unsupported field type: slice of", field.FieldSlice.Item.Type)
				}
			} else if field.Type == model.TypeBool {
				flags = append(flags, &cli.BoolFlag{
					Name:     prefix + mni.ToLowerCamel(fieldName),
					Required: field.Required,
				})
			} else if field.Type == model.TypeStruct {
				flags = r.appendFlags(flags, prefix+mni.ToLowerCamel(fieldName)+".", &field.FieldStruct.Model, update)
			} else if field.Type == model.TypeUint {
				flags = append(flags, &cli.UintFlag{
					Name:     prefix + mni.ToLowerCamel(fieldName),
					Required: field.Required,
				})
			} else if field.Type == model.TypeInt {
				flags = append(flags, &cli.IntFlag{
					Name:     prefix + mni.ToLowerCamel(fieldName),
					Required: field.Required,
				})
			} else if field.Type == model.TypeFloat {
				flags = append(flags, &cli.Float64Flag{
					Name:     prefix + mni.ToLowerCamel(fieldName),
					Required: field.Required,
				})
			} else if field.Type == model.TypeTime {
				fmt.Println("Unsupported field type: Time")
			} else {
				fmt.Println("Unsupported field type:", field.Type)
			}
		}
	}

	return flags
}

func (r RestCommandFactory) displaySingle(ctx *cli.Context, response *client.SingletonResponse) {
	t := table.NewWriter()
	t.SetOutputMirror(ctx.App.Writer)

	t.AppendHeader(table.Row{"Field", "Value"})
	r.appendSingle(t, "", response.Body, r.model)
	t.Render()
}

func (r RestCommandFactory) appendSingle(t table.Writer, prefix string, obj map[string]interface{}, m *model.Model) {
	for _, fieldName := range m.FieldsOrder {
		jsonFieldName := mni.ToLowerCamel(fieldName)
		field := m.Fields[fieldName]

		if field.WriteOnly {
			continue
		}

		if _, ok := obj[jsonFieldName]; !ok {
			continue
		}

		if field.Type == model.TypeStruct {
			r.appendSingle(t, prefix+fieldName+".", obj[jsonFieldName].(map[string]interface{}), &field.FieldStruct.Model)
		} else if field.Type == model.TypeSlice {
			values := obj[jsonFieldName].([]interface{})
			for i, value := range values {
				if field.FieldSlice.Item.Type == model.TypeStruct {
					valueMap := value.(map[string]interface{})
					r.appendSingle(t, prefix+fieldName+"["+strconv.Itoa(i)+"].", valueMap, &field.FieldSlice.Item.FieldStruct.Model)
				} else {
					t.AppendRow(table.Row{prefix + fieldName + "[" + strconv.Itoa(i) + "]", value})
				}
			}
		} else {
			t.AppendRow(table.Row{prefix + fieldName, obj[jsonFieldName]})
		}
	}
}

func (r RestCommandFactory) displayMultiple(ctx *cli.Context, response *client.MultitonResponse) {
	t := table.NewWriter()
	t.SetOutputMirror(ctx.App.Writer)

	t.AppendHeader(r.appendHeaderMultiple(table.Row{}, "", r.model))

	for _, obj := range response.Body {
		t.AppendRow(r.appendMultiple(table.Row{}, "", obj, r.model))
	}

	t.Render()
}

func (r RestCommandFactory) appendHeaderMultiple(header table.Row, prefix string, m *model.Model) table.Row {
	for _, fieldName := range m.FieldsOrder {
		field := m.Fields[fieldName]
		if !field.WriteOnly && field.Short {
			if field.Type == model.TypeStruct {
				header = r.appendHeaderMultiple(header, prefix+fieldName+".", &field.FieldStruct.Model)
			} else if field.Type == model.TypeSlice {
				if field.FieldSlice.Item.Type == model.TypeStruct {
					fmt.Println("Not implemented")
				} else {
					header = append(header, prefix+fieldName)
				}
			} else {
				header = append(header, prefix+fieldName)
			}
		}
	}
	return header
}

func (r RestCommandFactory) appendMultiple(row table.Row, prefix string, obj map[string]interface{}, m *model.Model) table.Row {
	for _, fieldName := range m.FieldsOrder {
		jsonFieldName := mni.ToLowerCamel(fieldName)
		field := m.Fields[fieldName]

		if field.WriteOnly || !field.Short {
			continue
		}

		if _, ok := obj[jsonFieldName]; !ok {
			continue
		}

		if field.Type == model.TypeStruct {
			row = r.appendMultiple(row, prefix+fieldName+".", obj[jsonFieldName].(map[string]interface{}), &field.FieldStruct.Model)
		} else if field.Type == model.TypeSlice {
			if field.FieldSlice.Item.Type == model.TypeStruct {
				fmt.Println("Not implemented")
			} else {
				row = append(row, obj[jsonFieldName])
			}
		} else {
			row = append(row, obj[jsonFieldName])
		}
	}
	return row
}

func NewRestCommandFactory(rootUrl, apiVersion, resourceName string, model *model.Model) CommandFactory {
	return &RestCommandFactory{
		rootUrl:      rootUrl,
		apiVersion:   apiVersion,
		resourceName: resourceName,
		model:        model,
	}
}
