package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import org.apache.commons.io.FileUtils;
import org.chengbing.dao.*;
import org.chengbing.entity.*;
import org.chengbing.service.IPhotoService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import java.io.File;
import java.io.IOException;
import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class PhotoServiceImpl extends ServiceImpl<PhotoMapper, Photo> implements IPhotoService {

    @Resource
    PhotoMapper mapper;

    @Resource
    UserMapper userMapper;

    @Resource
    TagPhotoMapper tagPhotoMapper;

    @Resource
    GalleryPhotoMapper galleryPhotoMapper;

    @Resource
    NamespacePhotoMapper namespacePhotoMapper;

    @Resource
    NamespaceMapper namespaceMapper;

    @Value("${file.uploadFolder}")
    String uploadFolder;

    @Override
    public Integer insertPhoto(Integer userId, MultipartFile file, Photo photo) {
        String uuid = userMapper.selectById(userId).getUuid();
        String fileName = file.getOriginalFilename();
        String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
        Integer visibility = photo.getVisibility();
        if (visibility == null || visibility < 0 || visibility > 3)
            photo.setVisibility(0);
        String photoUUID = (UUID.randomUUID().toString());
        photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
        photo.setPhotoUuid(photoUUID);
        photo.setFormat(suffix);
        photo.setToken(UUID.randomUUID().toString());
        photo.setUploadDate(LocalDateTime.now());
        photo.setUserId(userId);

        // String fileType = file.getContentType();
        String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
        File file1 = new File(folderLoc);
        if (!file1.exists())
            file1.mkdir();
        String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
        File photoFile = new File(localPath);
        try {
            file.transferTo(photoFile);
            return mapper.insert(photo);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public Integer insertPhotos(Integer userId, MultipartFile[] files, List<Photo> photos) {
        String uuid = userMapper.selectById(userId).getUuid();
        int total = photos.size();
        if (total != files.length)
            return -1;
        int change = 0;
        for (int i = 0; i < total; i ++)
        {
            MultipartFile file = files[i];
            Photo photo = photos.get(i);
            String fileName = file.getOriginalFilename();
            String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
            Integer visibility = photo.getVisibility();
            if (visibility == null || visibility < 0 || visibility > 3)
                photo.setVisibility(0);
            String photoUUID = (UUID.randomUUID().toString());
            photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
            photo.setPhotoUuid(photoUUID);
            photo.setFormat(suffix);
            photo.setToken(UUID.randomUUID().toString());
            photo.setUploadDate(LocalDateTime.now());
            photo.setUserId(userId);

            // String fileType = file.getContentType();
            String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
            File file1 = new File(folderLoc);
            if (!file1.exists())
                file1.mkdir();
            String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
            File photoFile = new File(localPath);
            try {
                file.transferTo(photoFile);
                change += mapper.insert(photo);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
        return change;
    }

    @Override
    public Integer deletePhoto(Integer userId, Integer photoId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected!=null)
        {
            String uuid = userMapper.selectById(userId).getUuid();
            Photo photo = mapper.selectById(photoId);
            String photoUUID = photo.getPhotoUuid();
            String suffix = photo.getFormat();
            String localPath = uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + photoUUID + "." + suffix;

            File photoFile = new File(localPath);
            boolean isDeleted = photoFile.delete();
            if (isDeleted)
                return mapper.deleteById(photoId);
        }
        return -1;
    }

    @Override
    public Integer updatePhotoNameAndVisibility(Integer userId, Photo photo) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photo.getPhotoId());
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected != null)
        {
            UpdateWrapper<Photo> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("photo_id", photo.getPhotoId());
            updateWrapper.set("photo_name", photo.getPhotoName());
            updateWrapper.set("visibility", photo.getVisibility());
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }

    @Override
    public List<Photo> queryPhotoByNamespace(Integer userId, Integer nsId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        wrapper.eq("ns_id", nsId);
        return mapper.selectList(wrapper);
    }

    @Override
    public Integer changeNamespace(Integer userId, Integer photoId, Integer nsId) {
        /*QueryWrapper<NamespacePhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        NamespacePhoto namespacePhoto = namespacePhotoMapper.selectOne(wrapper);
        namespacePhoto.setNsId(nsId);
        namespacePhotoMapper.updateById(namespacePhoto);*/

        QueryWrapper<Namespace> namespaceQueryWrapper = new QueryWrapper<>();
        namespaceQueryWrapper.eq("ns_id", nsId);
        namespaceQueryWrapper.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(namespaceQueryWrapper);

        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (namespace != null && selected != null)
        {
            UpdateWrapper<Photo> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("photo_id", photoId);
            updateWrapper.set("ns_id", nsId);
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }


    @Override
    public List<Photo> queryPhotoByTags(Integer userId, List<Integer> tagIds) {
        return tagPhotoMapper.selectByTags(userId, tagIds, tagIds.size());
    }

    @Override
    public Integer addTags(Integer userId, Integer photoId, List<Integer> tagIds) {
        int res = 0;
        for (Integer tagId : tagIds)
        {
            QueryWrapper<TagPhoto> wrapper = new QueryWrapper<>();
            wrapper.eq("tag_id", tagId);
            wrapper.eq("user_id", userId);
            wrapper.eq("photo_id", photoId);
            if (tagPhotoMapper.selectOne(wrapper) == null)
                res += tagPhotoMapper.insert(new TagPhoto(userId, tagId, photoId));
        }
        return res;
    }

    @Override
    public Integer deleteTag(Integer userId, Integer photoId, Integer tagId) {
        QueryWrapper<TagPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("tag_id", tagId);
        wrapper.eq("user_id", userId);
        wrapper.eq("photo_id", photoId);
        return tagPhotoMapper.delete(wrapper);
    }

    @Override
    public List<Photo> queryPhotoByGallery(Integer userId, Integer gaId) {
        return galleryPhotoMapper.selectPhotoByGallery(gaId);
    }

    @Override
    public Integer addToGallery(Integer userId, Integer photoId, Integer gaId) {
        QueryWrapper<GalleryPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("photo_id", photoId);
        if (galleryPhotoMapper.selectOne(wrapper) == null)
            return galleryPhotoMapper.insert(new GalleryPhoto(gaId, photoId));
        return -1;
    }

    @Override
    public Integer deleteFromGallery(Integer userId, Integer photoId, Integer gaId) {
        QueryWrapper<GalleryPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("photo_id", photoId);
        return galleryPhotoMapper.delete(wrapper);
    }
}
