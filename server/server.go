package main

import (
	pb "gRPC_server/proto"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

const (
	// Address 监听地址
	Address string = ":8083"
	// Network 网络通信协议
	Network string = "tcp"

	Path string = "./"
)

// 定义服务
type StreamService struct {
	pb.UnimplementedStreamServer
}

// 下载文件
type FileService struct {
	pb.UnimplementedFileServiceServer
}

func (s *StreamService) Conversations(srv pb.Stream_ConversationsServer) error {
	n := 1
	for {
		req, err := srv.Recv()
		if err != nil {
			return err
		}
		if err == io.EOF {
			return nil
		}
		err = srv.Send(&pb.StreamResponse{Answer: "from stream server answer: the " + strconv.Itoa(n) + " question is " + req.Question})
		if err != nil {
			return err
		}
		n++
		log.Printf("from stream client question: %s", req.Question)
	}
}

func (s *FileService) DownloadFile(fileReq *pb.DownloadFileRequest, srv pb.FileService_DownloadFileServer) error {
	filePath := fileReq.FilePath
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
				break
			} else {
				log.Println("文件读取错误err：", err)
				return err
			}
		}

		//发送数据
		//log.Println("发送数据，length: ", length)
		var blockInfo pb.DownloadFileResponse
		blockInfo.Content = buf[:length]
		blockInfo.FileName = []byte(filePath)
		blockInfo.Off = int64(offSet)
		blockInfo.Len = int32(length)
		senderr := srv.Send(&blockInfo)
		offSet = offSet + int64(length)
		if senderr != nil {
			if senderr == io.EOF {
				log.Println("发送到达文件末尾")
				break
			}
			log.Println("senderr:", senderr)
			return err
		} else {
			log.Println("senderr nil")
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

func (s *FileService) UploadFile(srv pb.FileService_UploadFileServer) error {
	// 接收文件
	var pFile *os.File
	var recv = &pb.UploadRequest{}
	var destFielName string
	for {
		err := srv.RecvMsg(recv)
		// 检查是否接收完成
		if err != nil {
			if err == io.EOF {
				log.Println("接收完成")
				break
			} else {
				log.Println("流消息接收失败err：", err)
				// 关闭文件
				defer func() {
					if pFile != nil {
						pFile.Close()
					}
				}()
				return err
			}
		}
		if recv != nil {
			destFielName = string(recv.FileName)
			_, destFielName = filepath.Split(destFielName)
			destFielName = Path + destFielName
			// 偏移检查
			if recv.Off == 0 {
				// 打开/创建文件
				pFile, err = os.OpenFile(destFielName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
				if err != nil {
					log.Println("打开目标文件错误: ", destFielName, ";err: ", err)
					return err
				}
			}
			// 数据写入
			_, err = pFile.WriteAt(recv.Content[:], recv.Off)
			if err != nil {
				log.Println("文件", destFielName, "写入失败，err", err)
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
	srv.SendAndClose(&pb.UploadResponse{FilePath: destFielName})
	return nil
}

func Server() {
	listener, err := net.Listen(Network, Address)
	if err != nil {
		log.Fatalf("net.Listen err: %v", err)
	}
	log.Println(Address + " net.Listing...")
	// 新建gRPC服务器实例
	grpcServer := grpc.NewServer()
	// 在gRPC服务器注册我们的服务
	//pb.RegisterStreamServer(grpcServer, &StreamService{})
	pb.RegisterFileServiceServer(grpcServer, &FileService{})
	//用服务器 Serve() 方法以及我们的端口信息区实现阻塞等待，直到进程被杀死或者 Stop() 被调用
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("grpcServer.Serve err: %v", err)
	}
	defer grpcServer.Stop()
}

func main() {
	Server()
}
