package org.chengbing.dao;

import org.apache.ibatis.annotations.Mapper;
import org.chengbing.entity.GalleryPhoto;
import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import org.chengbing.entity.Photo;

import java.util.List;

***REMOVED***
 * <p>
 *  Mapper 接口
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
@Mapper
public interface GalleryPhotoMapper extends BaseMapper<GalleryPhoto> {
    List<Photo> selectPhotoByGallery(Integer gaId);
***REMOVED***
