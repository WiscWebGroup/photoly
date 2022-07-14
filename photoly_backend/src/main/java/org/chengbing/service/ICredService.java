package org.chengbing.service;

import org.chengbing.entity.Cred;
import com.baomidou.mybatisplus.extension.service.IService;
import org.chengbing.entity.Namespace;
import org.chengbing.entity.Photo;

import javax.naming.Name;
import java.util.List;

***REMOVED***
 * <p>
 *  服务类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
***REMOVED***
public interface ICredService extends IService<Cred> {
    Integer createNewAPI(Integer userId, Cred cred);
    List<Cred> queryAPI(Integer userId);
    Integer updateAPI(Integer userId, Cred cred);
    Integer deleteAPI(Integer userId, Integer credId);

    Integer getUserId(String token);
    String getAuth(String token);
    byte[] render(Integer userId, String uuid);
    Integer deletePhoto(Integer userId, String uuid);

    Namespace queryRootNamespace(Integer userId);
    List<Namespace> queryNamespaces(Integer userId, Integer parentId);
    List<Photo> queryPhotoList(Integer userId, Integer nsId);
***REMOVED***
