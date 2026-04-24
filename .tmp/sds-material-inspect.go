package main
import (
 "context"; "encoding/json"; "fmt"; "log"; "task-processor/internal/sds/client"; "task-processor/internal/sds/design"
)
func main(){ c,err:=client.New(client.DefaultConfig()); if err!=nil{log.Fatal(err)}; s:=design.NewService(c); mats,err:=s.FindMaterialsByIDs(context.Background(), design.FindMaterialsRequest{IDs: []int64{459521446}, Fields:"id,name,imgUrl,width,height,file_code,content_type,photo_service_file_url,design_url,sourceType"}); if err!=nil{log.Fatal(err)}; b,_:=json.MarshalIndent(mats,"","  "); fmt.Println(string(b)) }
