package org.chengbing.service;

import org.apache.ibatis.annotations.Param;
import org.chengbing.entity.Photo;
import org.chengbing.entity.Tag;
import org.chengbing.entity.TagPhoto;
import com.baomidou.mybatisplus.extension.service.IService;

import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
public interface ITagPhotoService extends IService<TagPhoto> {
    List<Photo> queryPhotoByTagName(Integer userId, String tagName);
}
