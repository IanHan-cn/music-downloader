package music

import (
	"io"
	"log/slog"
	"strings"

	"github.com/IanHan-cn/music-downloader/tools"
	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

func parseFlacFile(file io.Reader, f FileInfo, lyrics string) (*flac.File, error) {
	flacFile, err := flac.ParseBytes(file)
	if err != nil {
		slog.Error("解析 FLAC 文件失败", "error", err)
		return nil, err
	}

	var cmt *flacvorbis.MetaDataBlockVorbisComment
	cmtIdx := -1
	for idx, meta := range flacFile.Meta {
		if meta.Type == flac.VorbisComment {
			cmt, err = flacvorbis.ParseFromMetaDataBlock(*meta)
			if err != nil {
				slog.Error("解析 Vorbis 标签失败", "error", err)
				return nil, err
			}
			cmtIdx = idx
			break
		}
	}

	if cmt == nil {
		cmt = flacvorbis.New()
	}

	_ = cmt.Add(flacvorbis.FIELD_TITLE, f.Title)
	for _, artist := range strings.Split(f.Artist, ",") {
		if artist = strings.TrimSpace(artist); artist != "" {
			_ = cmt.Add(flacvorbis.FIELD_ARTIST, artist)
		}
	}
	_ = cmt.Add(flacvorbis.FIELD_ALBUM, f.Album)
	if lyrics != "" {
		_ = cmt.Add("LYRICS", lyrics)
	}

	cmtMeta := cmt.Marshal()
	if cmtIdx >= 0 {
		flacFile.Meta[cmtIdx] = &cmtMeta
	} else {
		flacFile.Meta = append([]*flac.MetaDataBlock{&cmtMeta}, flacFile.Meta...)
	}
	return flacFile, nil
}

func addCoverToFlacFile(flacFile *flac.File, imgData []byte) (*flac.File, error) {
	mime, err := tools.DetectImageMime(imgData)
	picMeta, err := flacpicture.NewFromImageData(
		flacpicture.PictureTypeFrontCover,
		"Cover",
		imgData,
		mime,
	)
	if err != nil {
		slog.Error("创建封面块失败", "error", err)
		return nil, err
	}

	pictureMetaBlock := picMeta.Marshal()
	flacFile.Meta = append(flacFile.Meta, &pictureMetaBlock)
	return flacFile, nil
}
