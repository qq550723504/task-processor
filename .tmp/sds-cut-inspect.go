package main
import (
 "context"; "encoding/json"; "fmt"; "log"; "task-processor/internal/sds/client"; "task-processor/internal/sds/design"
)
func main(){ c,err:=client.New(client.DefaultConfig()); if err!=nil{log.Fatal(err)}; s:=design.NewService(c); page,err:=s.GetDesignProduct(context.Background(),212097); if err!=nil{log.Fatal(err)}; codes:=[]string{}; for _,p:=range page.PSDs{codes=append(codes,p.FileCode)}; m,err:=s.GetCutFileContent(context.Background(),codes); if err!=nil{log.Fatal(err)}; b,_:=json.MarshalIndent(m,"","  "); fmt.Println(string(b)) }
