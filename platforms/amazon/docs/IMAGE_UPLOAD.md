求
 图片要 Amazon1) -p/188/helcom/gpazon.rcentral.amps://sellettde](h Images Guion图
- [Amazd) - 开发路线../ROADMAP.mDMAP.md](- [ROA文档
md) - 属性映射APPING.ATTRIBUTE_M(./PING.md]IBUTE_MAP
- [ATTR 相关文档
# 🔗量会产生费用

#3 存储和流控制**: S传超时
4. **成本件大小，避免上图片文**: 注意控制片大小3. **图，需要稳定网络
on S3 在国外国内访问，Amaz: 1688 图片在. **网络环境**cket
2 凭证和 S3 Bu置 AWS须正确配配置**: 必 **S3 事项

1.# 🚨 注意反爬虫机制

#限制**: 避免触发 **速率载
4.败的下制**: 自动重试失. **重试机免大量内存占用
3理，避: 流式处2. **内存优化**支持并发下载多张图片
 **批量下载**: 性能优化

1.```

## 📊 /s3
/services-sdk-go-v2b.com/aws/awithu       └── goader
  S3Upling
    └──ion/imagat/disintegrb.com   └── githur
    │somageProces ├── I通用下载器)
   er (oadnlageDowloader.Im/downon comm│   └──er
    nloadmageDow├── I
    ageHandler

```
Im## 🔍 依赖关系误

回错全部失败: 返: 使用原图
- 
- 处理失败他图片 跳过该图片，继续处理其
- 下载失败: 错误处理
###: 95%

- JPEG 质量IF持 JPEG、PNG、G始格式
- 支转换
- 自动检测原格式### 

量）czos 算法（高质- 使用 Lan
- 保持宽高比
0 像素: 2000x200
- 目标尺寸调整

### 尺寸 自动截取前 9 张 张图片
-azon 最多支持 9
- Am

### 数量限制则## ⚙️ 图片处理规

```r.jpg"
}
veank/co/img/ib1.alicdn.com/cbu0ps:/htt: "ainImage"
  "mn.jpg",maig/ibank//imcom.alicdn.tps://cbu01Url": "ht "image
  ],
 jpg"/ibank/xxx2./imglicdn.com//cbu01.a"https: pg",
   1.jbank/xxxdn.com/img/icbu01.alic"https://
    ges": ["iman
{
  ：
```jso主图

示例数据- ge** (字符串) . **mainIma) - 单张主图
3 (字符串*imageUrl**- 多张图片
2. *** (数组) 
1. **images段优先级：
中提取图片的字从 1688 产品数据688 图片字段

🎯 1
## ")
```
e/jpeg"imag", resized, g.jp123/imagects/dud(ctx, "proloader.Uploa up3URL, err :=bucket")
s"my-ent, der(s3CliNewS3Uploa= service. :
uploader传到S3

// 上000, 2000)geData, 2imar.Resize(ocesso= prsized, err :sor()
recesewImagePro= service.N
processor : 处理图片
//.jpg")
magemple.com/ips://exa"httad(oader.Downlorr := downlata, egeDader()
imaownloewImageDservice.Nader := 载图片
downlo```go
// 下处理图片


### 手动`
)
``_image_url"tData("mainGe_ := ctx.RL, 
mainImageU)age_urls"ta("imDa= ctx.Gets, _ :
imageURL果/ 5. 获取结le(ctx)

/Handa)
handler.productDat_data", uctraw_prodetData("tx.Sext()
cwTaskCont= amazon.Ne
ctx :执行处理
// 4. ler)
handHandler(eline.Addine中使用
pipPipel)

// 3. 在aderploHandler(us.NewImage := handler器
handler创建图片处理/ 2. 
/")
ket"my-buclient, s3C3Uploader(ice.NewServ:= s端
uploader S3 客户AWS 建 = // ... 创ient :Cl3上传器
s3o
// 1. 创建S
```g本使用
## 基📝 使用示例

#件（可选）

## ect` - 删除文:DeleteObj）
- `s3件（可选bject` - 读取文- `s3:GetO传文件
bject` - 上s3:PutO `以下权限：
-t 需要
S3 Bucke
 AWS 权限##

#``_KEY"
`SECRETYOUR_"ess_key: cc   secret_aY"
 _ACCESS_KEy_id: "YOURccess_ke    a
  aws:
"s-east-1n: "u regioges"
   -imaazon-product "your-am   bucket:3:
 zon:
  sma
```yaml
a3 相关配置：
添加 S在配置文件中 配置

需要要求

### S3 配置
## 🔧
```
 主图URLge_url"] ="main_imaxt[nte URLs
Co"] = S3"image_urlst[↓
Contexr)
    Uploade上传S3 (S3
    ↓or)
ocessePrmag图片 (I↓
处理   oader)
 ageDownl载图片 (Im
    ↓
下e)ImagageUrl, mainimRL (images, 提取图片U↓
    "]
t_data"raw_producContext[`
*:
``文

**数据流*L到上下 保存S3 UR传到S3
5. 上格式）
4.理图片（调整大小、验证
3. 处URL
2. 下载图片. 从产品数据提取图片:
1**处理步骤**流程

整个图片处理**功能**: 协调`

handler.goers/image_handlzon/ms/amatforla
**文件**: `p处理Handler)图片eHandler (mag. I 4###
```

), errortringe) ([]ss [][]bytg, imagein strtext, prefixcontext.Contx dMultiple(cer) Uploaload(u *S3Up
func r)ring, errog) (stntType strin]byte, conteg, data [t, key strint.Contexontexx c Upload(ct*S3Uploader)go
func (u ``
`

**方法**: S3 URL名生成
- 返回一文件测
- 唯动内容类型检持
- 自**:
- 批量上传支S3

**特性mazon : 上传图片到 A

**功能**er.go`adce/s3_uploazon/servi/amtforms文件**: `pla上传器)
**(S33Uploader 

### 3. SF, GI式: JPEG, PNG
- 支持格10MB: 文件大小像素
- 最大: 2000x2000 推荐尺寸00 像素
- 10最小尺寸: 1000x*:
- 片要求**Amazon 图

*)
```rormageInfo, er []byte) (*IimageDatao(mageInfsor) GetIoces (p *ImagePr
functe) errorbymageData []at(ilidateFormcessor) VamageProc (p *Ifun error)
yte, int) ([]bdth, heighta []byte, wiimageDat) Resize(cessormagePro
func (p *I``go**方法**:
`
- 格式验证

比）
- 图片质量优化动调整图片大小（保持宽高
- 自格式G、GIF EG、PN支持 JP**特性**:
- 
调整
尺寸**: 图片格式验证和*功能go`

*essor.procrvice/image_zon/seatforms/ama*: `pl)
**文件*处理器sor (图片roces# 2. ImageP
##
```
te, error)) ([][]bystringls [](urultipleownloadMownloader) Dc (d *ImageDr)
fun, errog) ([]bytel strinownload(urloader) D *ImageDowno
func (d`g法**:
``控检测

**方- 风
- 速率限制
nt
- 自动重试机制 User-Age持多样化的增强下载器
- 支r.go` e_downloadenloader/imag `common/dow*:
- 使用**特性*支持增强反风控

 封装通用图片下载器，

**功能**:`r.gode_downloavice/imageserzon/rms/ama `platfo*:)
**文件*er (图片下载器eDownloadag
### 1. Im# 📦 核心组件
L)
```

#S3 UR引用Listing (n    ↓
Amazo3)
 der (上传到SUploa ↓
S3 (处理图片)
   rocessorePImag    ↓
)
loader (下载图片
ImageDownRL
    ↓`
1688图片U

``流程数据 🔄 上传流程。

##azon S3 的完整品图片到 Am了从 1688 产
实现 📋 功能概述
片上传功能

##n 图# Amazo