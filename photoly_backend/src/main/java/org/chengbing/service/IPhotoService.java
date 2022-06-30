package org.chengbing.service;

import org.chengbing.entity.Photo;
import com.baomidou.mybatisplus.extension.service.IService;
import org.chengbing.util.Result;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.multipart.MultipartFile;

import javax.servlet.http.HttpServletRequest;
import java.util.List;

/**
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
public interface IPhotoService extends IService<Photo> {
    Integer insertPhoto(Integer userId, MultipartFile file, @RequestBody Photo photo);
    Integer deletePhoto(Integer userId, Integer photoId);
    Integer updatePhotoNameAndVisibility(Integer userId, @RequestBody Photo photo);
    List<Photo> queryPhotoByNamespace(Integer userId, Integer nsId);
    Integer changeNamespace(Integer userId, Integer photoId, Integer nsId);
    List<Photo> queryPhotoByTags(Integer userId, List<Integer> tagIds);
    Integer addTags(Integer userId, Integer photoId, List<Integer> tagIds);
    Integer deleteTag(Integer userId, Integer photoId, Integer tagId);
    List<Photo> queryPhotoByGallery(Integer userId, Integer gaId);
    Integer addToGallery(Integer userId, Integer photoId, Integer gaId);
    Integer deleteFromGallery(Integer userId, Integer photoId, Integer gaId);
}
