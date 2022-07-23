package org.chengbing.service;

import org.chengbing.entity.Namespace;
import com.baomidou.mybatisplus.extension.service.IService;
import org.chengbing.util.Result;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;

import javax.servlet.http.HttpServletRequest;
import java.util.List;

***REMOVED***
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
public interface INamespaceService extends IService<Namespace> {
    Integer insertNamespace(String name, Integer parentId, Integer userId);
    Namespace queryRootNamespace(Integer userId);

    List<Namespace> queryNamespace(Integer userId, Integer parentId);

    Integer updateNamespaceName(int userId, Namespace namespace);

    Integer updateNamespaceParent(int userId, Namespace namespace);

    Integer insertNamespace(Integer userId, Namespace namespace);
    Integer deleteNamespace(Integer userId, Integer nsId);

    Integer deletePhotoHelper(Integer userId, Integer nsId, String userUUID);

***REMOVED***
