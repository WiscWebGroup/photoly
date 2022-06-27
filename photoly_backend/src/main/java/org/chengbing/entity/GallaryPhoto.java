package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="GallaryPhoto对象", description="")
public class GallaryPhoto implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "gp_id", type = IdType.AUTO)
    private Integer gpId;

    private Integer gaId;

    private Integer photoId;


}
