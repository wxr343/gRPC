package main

import (
	"context"
	pb "gRPC_client/proto"
	tlsconfig "gRPC_client/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Address 连接地址
const Address string = "127.0.0.1:8083"
const DownloadFilePath string = "D:\\GO\\goworkspace\\src\\goStudy\\gRPC\\ReadMe.txt"
const UploadFilePath string = "./upload.txt"

var streamClient pb.StreamClient
var downloadClient pb.FileServiceClient
var uploadClient pb.FileServiceClient

var downloadPath = pb.DownloadFileRequest{FilePath: DownloadFilePath}

// tls配置
func TlsSet() (opts []grpc.DialOption) {
	var caFile *string
	var serverHostOverride *string
	var tls bool = false
	// 配置tls
	if tls {
		if *caFile == "" {
			*caFile = tlsconfig.Path("x509/ca_cert.pem")
		}
		creds, err := credentials.NewClientTLSFromFile(*caFile, *serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	return
}

func getConnect() (conn *grpc.ClientConn) {
	// 配置tls
	opts := TlsSet()
	// 连接服务器
	conn, err := grpc.NewClient(Address, opts...)
	if err != nil {
		log.Fatalf("net.Connect err: %v", err)
	}
	return conn
}

// conversations 调用服务端的Conversations方法
func conversations() {
	//调用服务端的Conversations方法，获取流
	stream, err := streamClient.Conversations(context.Background())
	if err != nil {
		log.Fatalf("get conversations stream err: %v", err)
	}
	for n := 0; n < 5; n++ {
		err := stream.Send(&pb.StreamRequest{Question: "stream client rpc " + strconv.Itoa(n)})
		if err != nil {
			log.Fatalf("stream request err: %v", err)
		}
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Conversations get stream err: %v", err)
		}
		// 打印返回值
		log.Println(res.Answer)
	}

	//最后关闭流
	err = stream.CloseSend()
	if err != nil {
		log.Fatalf("Conversations close stream err: %v", err)
	}
}

// 下载
func download(destFielName string, conn *grpc.ClientConn) {
	// 建立单向流下载gRPC
	downloadClient = pb.NewFileServiceClient(conn)

	// 获取文件传输流
	block, err := downloadClient.DownloadFile(context.Background(), &downloadPath)
	if err != nil {
		log.Println("获取流传输对象错误，err:", err)
		return
	}

	// 接收文件
	var pFile *os.File
	var recv = &pb.DownloadFileResponse{}

	for {
		err = block.RecvMsg(recv)
		// 检查是否接收完成
		if err != nil {
			if err == io.EOF {
				break
			} else {
				log.Println("流消息接收失败err：", err)
				return
			}
		}
		if recv != nil {
			// 偏移检查
			if recv.Off == 0 {
				// 打开/创建文件
				pFile, err = os.OpenFile(destFielName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					log.Println("打开目标文件错误: ", destFielName, ";err: ", err)
					return
				}
			}
			// 数据写入
			_, err = pFile.WriteAt(recv.Content[:], recv.Off)
			if err != nil {
				log.Println("文件", destFielName, "写入失败，err", err)
				return
			}

		}

	}
	// 关闭文件
	defer func() {
		if pFile != nil {
			pFile.Close()
		}
	}()
}

// 上传
func upload(uploadFile string, conn *grpc.ClientConn) error {
	filePath := uploadFile
	// 判断文件/文件夹是否存在
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return err
	}
	// 判断是否为文件
	if info.IsDir() {
		log.Println("filePath为文件夹，非文件")
		return err

	}
	// 打开文件
	//log.Println("打开文件", filePath)
	pFile, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
	if err != nil {
		log.Println("打开文件", filePath, "失败，err: ", err)
		return err
	}

	// 建立连接
	uploadClient = pb.NewFileServiceClient(conn)
	// 获取文件传输流
	block, err := uploadClient.UploadFile(context.Background())
	if err != nil {
		log.Println("获取流传输对象错误，err:", err)
		return err
	}

	// 设置传输文件块大小2M
	var blockSize = 1024 * 1024 * 2
	// 定义每次读取缓冲
	var buf = make([]byte, blockSize)
	var offSet int64 = 0
	var length = 0
	for {
		// 从file类型中读取文件
		length, err = pFile.Read(buf)
		if err != nil {
			// 文件末尾结束,退出读取
			if err == io.EOF {
				log.Println("到达文件末尾")
				re, _ := block.CloseAndRecv()
				log.Println(re.FilePath)
				break
			} else {
				log.Println("文件读取错误err：", err)
				return err
			}
		}
		//发送数据
		log.Println("发送数据，length: ", length)
		var blockInfo pb.UploadRequest
		blockInfo.Content = buf[:length]
		blockInfo.FileName = []byte(filePath)
		blockInfo.Off = int64(offSet)
		blockInfo.Len = int32(length)
		//senderr := srv.Send(&blockInfo)
		senderr := block.Send(&blockInfo)
		offSet = offSet + int64(length)
		if senderr != nil {
			if senderr == io.EOF {
				// 上传完成
				log.Println("上传完成")
				break
			} else {
				log.Println("senderr:", senderr)
				return err
			}
		}

	}

	// 关闭文件
	defer func() {
		if pFile != nil {
			pFile.Close()
		}
	}()

	return nil
}

func main() {
	conn := getConnect()

	defer conn.Close()

	// 建立双向流gRPC连接
	//streamClient = pb.NewStreamClient(conn)
	//conversations()
	_, destFielName := filepath.Split(DownloadFilePath)
	destFielName = "./" + destFielName
	download(destFielName, conn)
	err := upload(UploadFilePath, conn)
	if err != nil {
		log.Println("上传错误：", err)
	}
}
