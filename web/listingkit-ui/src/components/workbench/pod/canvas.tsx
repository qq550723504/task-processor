"use client";
import {useEffect,useRef,useState} from "react";
import {Stage,Layer,Rect,Group,Image as CanvasImage} from "react-konva";
import type {PODApproval,PODManifest} from "@/lib/contracts/pod";
export type LayoutTransform={layerId:string;x:number;y:number;scale:number;angle:number};
export function fitGeometry(region:{width:number;height:number},image:{width:number;height:number},t:LayoutTransform){
 const factor=600/Math.max(region.width,region.height),w=region.width*factor,h=region.height*factor;
 const fit=Math.min(w/image.width,h/image.height)*t.scale;
 return {regionX:(600-w)/2,regionY:(600-h)/2,regionWidth:w,regionHeight:h,x:t.x*600,y:t.y*600,width:image.width*fit,height:image.height*fit};
}
export function DesignCanvas({region,image,transform,onChange,onReady,disabled}:{region:PODManifest["manifest"]["layers"][number];image:PODApproval["image"];transform:LayoutTransform;onChange:(t:LayoutTransform)=>void;onReady:(ready:boolean)=>void;disabled:boolean}){
 const host=useRef<HTMLDivElement>(null),[size,setSize]=useState(600),[bitmap,setBitmap]=useState<HTMLImageElement|null>(null);
 const ready=useRef(onReady);useEffect(()=>{ready.current=onReady},[onReady]);
 useEffect(()=>{const element=host.current;if(!element)return;const observer=new ResizeObserver(entries=>{const width=entries[0]?.contentRect.width;if(width)setSize(Math.min(600,Math.floor(width)))});observer.observe(element);return()=>observer.disconnect()},[]);
 useEffect(()=>{ready.current(false);const img=new window.Image();img.onload=()=>{if(img.naturalWidth===image.width&&img.naturalHeight===image.height){setBitmap(img);ready.current(true)}else{setBitmap(null)}};img.onerror=()=>{setBitmap(null);ready.current(false)};img.src=image.url;return()=>{img.onload=null;img.onerror=null}},[image.url,image.width,image.height]);
 const g=fitGeometry(region,image,transform);
 return <div ref={host} className="w-full max-w-[600px]" role="img" aria-label="图案布局预览；可使用旁边的数值控制调整位置、缩放和旋转"><Stage width={size} height={size} scaleX={size/600} scaleY={size/600}><Layer><Rect width={600} height={600} fill="#e5e7eb"/><Rect x={g.regionX} y={g.regionY} width={g.regionWidth} height={g.regionHeight} fill="white"/>
 <Group clipX={g.regionX} clipY={g.regionY} clipWidth={g.regionWidth} clipHeight={g.regionHeight}>{bitmap?<CanvasImage image={bitmap} x={g.x} y={g.y} width={g.width} height={g.height} offsetX={g.width/2} offsetY={g.height/2} rotation={transform.angle} draggable={!disabled} onDragEnd={e=>onChange({...transform,x:Math.max(0,Math.min(1,e.target.x()/600)),y:Math.max(0,Math.min(1,e.target.y()/600))})}/>:null}</Group>
 <Rect x={g.regionX} y={g.regionY} width={g.regionWidth} height={g.regionHeight} stroke="#64748b" strokeWidth={2} listening={false}/></Layer></Stage>{!bitmap?<p role="status">图案预览无法显示时，请重新读取已批准图案。</p>:null}</div>;
}
