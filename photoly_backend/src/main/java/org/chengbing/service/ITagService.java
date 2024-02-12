package org.chengbing.service;

import org.chengbing.entity.Tag;
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
public interface ITagService extends IService<Tag> {
    List<Tag> queryTag(Integer userId);

    ResultPage<List<Tag>> queryTagPage(Integer userId, Integer page, Integer rowsPerPage);
    Integer deleteTag(Integer userId, Integer tagId);
    Integer updateTag(int userId, Tag tag);
    Integer insertTag(Integer userId, Tag tag);
}
