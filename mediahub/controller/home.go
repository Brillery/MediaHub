package controller

import (
	"github.com/gin-gonic/gin"
	"math/rand"
	"net/http"
)

type home struct {
	Banners []string `json:"banners"`
	Images1 []string `json:"images1"`
	Images2 []string `json:"images2"`
}

var bannerDataList = []string{
	"https://img.0voice.com/public/ea9e8a4f224a2f04801d530346705995.jpg",
	"https://img.0voice.com/public/d7ec6513d12fe44db70403b850810d02.jpg",
	"https://img.0voice.com/public/b455edba25f0260b7261652c4d552785.jpg",
	"https://img.0voice.com/public/77502fb1baf9fa5bec801ded0b43cbbe.png",
	"https://img.0voice.com/public/fff9e293bced5d480a0f4b10c2e4b74d.jpg",
	"https://img.0voice.com/public/f26b5169463028090519c57556063865.JPG",
	"https://img.0voice.com/public/f08df16adcb44cebd61563cd48106f57.png",
	"https://img.0voice.com/public/04b8bc586337c8c9e96bf44fd2e01d5d.png",
	"https://img.0voice.com/public/13cc6cbc3efd2b13652f1096c52f491a.jpg",
	"https://img.0voice.com/public/147901baf3b651e253ee9cb027164f65.jpg",
	"https://img.0voice.com/public/2206b1c0d64f44033f9755815080ea9e.jpg",
	"https://img.0voice.com/public/3e79fc385ed84922a57904d4bb12de36.jpg",
	"https://img.0voice.com/public/42dab75ba153927739ec1cd0880f1b2d.png",
	"https://img.0voice.com/public/4b61b7ed8b0035b5b73b5b8fff6f1301.jpg",
	"https://img.0voice.com/public/4fc32ab6d2b008d9ef1d35419707bea9.png",
	"https://img.0voice.com/public/6a9ea9c60a934817720430ca3df36c4f.jpg",
	"https://img.0voice.com/public/6b4ce5aee35282ab26b1a2cc07487f88.png",
	"https://img.0voice.com/public/71eecf5a40cb5d10b51073f763c079c0.jpg",
	"https://img.0voice.com/public/73a89df0ba708c75540bb9ef85d6ebc8.png",
	"https://img.0voice.com/public/77684db5fd604f382be370a411349541.jpg",
	"https://img.0voice.com/public/7ae8ded66b51e4d56f90d9369e672ced.jpg",
	"https://img.0voice.com/public/8a68aebef3f648db5f0e270ecf15444b.jpg",
	"https://img.0voice.com/public/97243e1ec34455202e2f44aacb7424a5.png",
	"https://img.0voice.com/public/9a1ff8f3bb51601653a96d3b292978c0.png",
	"https://img.0voice.com/public/a6c2e0eec4a536e12701d380e6364eff.jpg",
	"https://img.0voice.com/public/bb0901ce7a4c50d4d20f3f3c3a8657e8.jpg",
	"https://img.0voice.com/public/d3ea38f203b31a331a479f641b653191.jpg",
	"https://img.0voice.com/public/df8befe8867b752478da1d50049c8499.jpg",
	"https://img.0voice.com/public/e3fdb3d848748c492e3385691b0e9cc1.jpg",
}

var imgDataList = []string{
	"https://img.0voice.com/public/195940c89673b1ce866a4651152c8156.jpg",
	"https://img.0voice.com/public/d7b7efc8395e97dc7a22581cc61a5e81.jpg",
	"https://img.0voice.com/public/fcea9fe6e3e03b28c9f63681db92ccaa.jpg",
	"https://img.0voice.com/public/b6bdc1da552e06185b183e22492dd59a.jpg",
	"https://img.0voice.com/public/75b836c452aba2e8aa8a798174400217.jpg",
	"https://img.0voice.com/public/aa11f2d4d45c45e35ff5379d6610df50.jpg",
	"https://img.0voice.com/public/2c96be6890d9feaf6ffbb42ad3001c63.jpg",
	"https://img.0voice.com/public/01adfac9a724d2e90f4aaa1b3a7006a5.jpg",
	"https://img.0voice.com/public/6ddc586100c406bb1b74a87e1b95f6b6.jpg",
	"https://img.0voice.com/public/073d145eda2c91870e4db611e19fb477.jpg",
	"https://img.0voice.com/public/c2dbdc57fc5cabda496814ecfdf122ff.jpg",
	"https://img.0voice.com/public/c0a2a1f352c28e796cb3c61d5cb1f51d.jpg",
	"https://img.0voice.com/public/f925ac543948d19739c1587883581d6b.jpg",
	"https://img.0voice.com/public/ac331309cefa7a8cc9cdb7403961f33a.png",
	"https://img.0voice.com/public/930756c26163d76ec5cf5bd17a0f7e6d.jpg",
	"https://img.0voice.com/public/a02acbb5b5c72065b7d31df748336ddc.jpg",
	"https://img.0voice.com/public/723a368d96934b2490c43c718d2342ce.jpg",
	"https://img.0voice.com/public/32fb4eda52ee8f31cbec8b6ca48d94e4.jpg",
	"https://img.0voice.com/public/8906227ba3f69df4ae086545b4254b79.jpg",
	"https://img.0voice.com/public/9c4a96a6ee3c29f559a38a50bf976f96.jpg",
	"https://img.0voice.com/public/683d2ca62565e32552963f63b82f74c1.jpg",
	"https://img.0voice.com/public/d6ac5a1fceabe5a0459dbeec7d96b807.jpg",
	"https://img.0voice.com/public/6132d707900e2ca27d23e6f0b5d6218f.jpg",
	"https://img.0voice.com/public/484bec910a68502ddce1136faec5a55e.jpg",
	"https://img.0voice.com/public/9cd671641e92642d240fce2f861475bb.png",
	"https://img.0voice.com/public/112a74cb2578a1c83d5934955f132f78.jpg",
	"https://img.0voice.com/public/54461b09aee83653128f0884122ed4ec.png",
	"https://img.0voice.com/public/76b824bbd1d1a275b80be668c694ad45.jpg",
	"https://img.0voice.com/public/0a44e10b644418b42bbee4013276fdb1.jpg",
	"https://img.0voice.com/public/23f4bbe22fbd93185a9a848590ff1bfa.jpg",
}

func (*Controller) Home(c *gin.Context) {
	h := &home{}
	bannerNum := 3
	indexList := make([]int, len(bannerDataList))
	for i, _ := range bannerDataList {
		indexList[i] = i
	}
	list := randList(indexList, bannerNum)
	bannerList := make([]string, bannerNum)
	for i := 0; i < bannerNum; i++ {
		bannerList[i] = bannerDataList[list[i]]
	}

	imgNum := 10
	indexList = make([]int, len(imgDataList))
	for i, _ := range imgDataList {
		indexList[i] = i
	}
	list = randList(indexList, imgNum)
	imgList := make([]string, imgNum)
	for i := 0; i < imgNum; i++ {
		imgList[i] = imgDataList[list[i]]
	}

	h.Banners = bannerList
	h.Images1 = imgList[:5]
	h.Images2 = imgList[5:]
	c.JSON(http.StatusOK, h)
}

func randList(indexList []int, num int) []int {
	list := make([]int, num)
	for i := 0; i < num; i++ {
		l := len(indexList)
		index := rand.Intn(l)
		list[i] = indexList[index]
		indexList = append(indexList[:index], indexList[index+1:]...)
	}
	return list
}
