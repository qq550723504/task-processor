package main
import (
 "fmt"; "image"; "image/color"; "github.com/disintegration/imaging"
)
func main(){ source:=image.NewNRGBA(image.Rect(0,0,40,40)); for y:=10;y<30;y++{for x:=10;x<30;x++{source.Set(x,y,color.NRGBA{R:200,G:30,B:20,A:255})}}
 design:=imaging.Fit(source,76,76,imaging.Lanczos); fmt.Printf("bounds=%v center=%+v p38=%+v p30=%+v p20=%+v\n",design.Bounds(), color.NRGBAModel.Convert(design.At(38,38)), color.NRGBAModel.Convert(design.At(38,38)), color.NRGBAModel.Convert(design.At(30,30)), color.NRGBAModel.Convert(design.At(20,20)))
}
