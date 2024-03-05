package org.chengbing.service;

import org.chengbing.entity.PhotoExtra;
import com.baomidou.mybatisplus.extension.service.IService;

import java.util.LinkedHashMap;
import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2024-03-04
 */
public interface IPhotoExtraService extends IService<PhotoExtra> {

    List<LinkedHashMap<String, Object>> queryGlobalPhoto(Integer userId);

    List<LinkedHashMap<String, Object>> queryGalleryPhoto(Integer userId, Integer gaId);

}
