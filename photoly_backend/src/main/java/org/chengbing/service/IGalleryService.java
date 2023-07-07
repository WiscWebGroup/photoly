package org.chengbing.service;

import org.chengbing.entity.Gallery;
import com.baomidou.mybatisplus.extension.service.IService;
import org.chengbing.util.ResultPage;

import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
public interface IGalleryService extends IService<Gallery> {
    Integer insertGallery(String name, Integer userId, String color);
    List<Gallery> queryGallery(Integer userId);
    ResultPage<List<Gallery>> queryGalleryPage(Integer userId, Integer page, Integer rowsPerPage);
    Integer deleteGallery(Integer userId, Integer gaId);
    Integer updateGallery(Integer userId, Integer gaId, Gallery gallery);
}
