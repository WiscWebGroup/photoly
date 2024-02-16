package org.chengbing.service.impl;

import org.chengbing.entity.GalleryPhoto;
import org.chengbing.dao.GalleryPhotoMapper;
import org.chengbing.entity.Photo;
import org.chengbing.service.IGalleryPhotoService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class GalleryPhotoServiceImpl extends ServiceImpl<GalleryPhotoMapper, GalleryPhoto> implements IGalleryPhotoService {
    @Resource
    GalleryPhotoMapper galleryPhotoMapper;
    @Override
    public List<Photo> queryPhotoByGaName(Integer userId, String gaName) {
        return galleryPhotoMapper.queryPhotoByGaName(userId, gaName);
    }
}
