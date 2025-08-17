package services

import "os"

type UploadFileService struct {
	baseDir string
}

func NewUploadFileService(baseDir string) *UploadFileService {
	return &UploadFileService{
		baseDir: baseDir,
	}
}

func (s *UploadFileService) SaveFile(filename string, data []byte) (string, error) {
	filePath := s.baseDir + "/" + filename
	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

func (s *UploadFileService) DeleteFile(filename string) error {
	filePath := s.baseDir + "/" + filename
	err := os.Remove(filePath)
	if err != nil {
		return err
	}
	return nil
}

func (s *UploadFileService) FileExists(filename string) bool {
	filePath := s.baseDir + "/" + filename
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func (s *UploadFileService) GetFilePath(filename string) string {
	filePath := s.baseDir + "/" + filename
	return filePath
}

func (s *UploadFileService) GetFileURL(filename string) string {
	filePath := s.baseDir + "/" + filename
	return "file://" + filePath // Adjust this based on your URL structure
}
