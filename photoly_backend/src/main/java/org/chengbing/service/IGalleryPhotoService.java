package org.chengbing.service;

import org.apache.ibatis.annotations.Param;
import org.chengbing.entity.GalleryPhoto;
import com.baomidou.mybatisplus.extension.service.IService;
import org.chengbing.entity.Photo;

import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
public interface IGalleryPhotoService extends IService<GalleryPhoto> {
    List<Photo> queryPhotoByGaName(Integer userId, String gaName);
}
